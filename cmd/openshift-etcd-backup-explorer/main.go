// Command openshift-etcd-backup-explorer inspects an OpenShift or Kubernetes etcd backup. It
// reads the snapshot file directly, so it needs no running etcd and never
// modifies the backup.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/api"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/index"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/webui"
)

// Build information, set by the release build with -ldflags. A build from
// source leaves them at these defaults.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const usage = `openshift-etcd-backup-explorer inspects an etcd backup taken from an OpenShift or Kubernetes cluster.

Usage:
  openshift-etcd-backup-explorer serve  [-backup PATH] [-addr ADDR] [-open]   start the web browser UI
  openshift-etcd-backup-explorer info   [-backup PATH]                        print snapshot and cluster facts
  openshift-etcd-backup-explorer kinds  [-backup PATH]                        list resource types and counts
  openshift-etcd-backup-explorer ls     [-backup PATH] -kind KIND [-namespace NS] [-q TEXT]
  openshift-etcd-backup-explorer get    [-backup PATH] -kind KIND -name NAME [-namespace NS] [-o yaml|json]
  openshift-etcd-backup-explorer export [-backup PATH] -kind KIND -out DIR [-namespace NS] [-q TEXT]
  openshift-etcd-backup-explorer demo   [-out DIR]                          write a synthetic backup to try
  openshift-etcd-backup-explorer version                                    print the build version

PATH may be a directory holding snapshot_*.db and static_kuberesources_*.tar.gz,
or a single snapshot .db file. It defaults to ./backup.

Examples:
  openshift-etcd-backup-explorer serve -open
  openshift-etcd-backup-explorer kinds | head -20
  openshift-etcd-backup-explorer ls -kind v1/Pod -namespace openshift-etcd
  openshift-etcd-backup-explorer get -kind v1/ConfigMap -namespace openshift-etcd -name etcd-pod -o yaml
  openshift-etcd-backup-explorer demo && openshift-etcd-backup-explorer serve -backup ./demo-backup -open
  openshift-etcd-backup-explorer export -kind PersistentVolume -out ./pvs
  openshift-etcd-backup-explorer export -kind Secret -namespace openshift-etcd -out - > secrets.yaml

Export removes metadata.uid, metadata.creationTimestamp and managedFields so the
manifests can be read or applied elsewhere. Pass -keep-generated to keep them.
`

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "serve":
		err = runServe(args, log)
	case "info":
		err = runInfo(args)
	case "kinds":
		err = runKinds(args)
	case "ls":
		err = runLs(args)
	case "get":
		err = runGet(args)
	case "export":
		err = runExport(args)
	case "demo":
		err = runDemo(args)
	case "version", "-v", "--version":
		fmt.Printf("openshift-etcd-backup-explorer %s (commit %s, built %s, %s/%s)\n",
			version, commit, date, runtime.GOOS, runtime.GOARCH)
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// loadOne discovers the backups at path and loads the newest one, which is
// what every terminal subcommand works against.
func loadOne(path string) (*index.Backup, error) {
	backups, err := index.Discover(path)
	if err != nil {
		return nil, err
	}
	if err := backups[0].Load(); err != nil {
		return nil, err
	}
	return backups[0], nil
}

func backupFlag(fs *flag.FlagSet) *string {
	return fs.String("backup", "./backup", "path to a backup directory or a snapshot .db file")
}

// ---- serve ----

func runServe(args []string, log *slog.Logger) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	path := backupFlag(fs)
	addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
	open := fs.Bool("open", false, "open the browser once the server is ready")
	if err := fs.Parse(args); err != nil {
		return err
	}

	backups, err := index.Discover(*path)
	if err != nil {
		return err
	}
	log.Info("found backups", "count", len(backups), "path", *path)

	// Load every backup up front so the UI can switch between them instantly.
	for _, b := range backups {
		start := time.Now()
		log.Info("indexing backup", "id", b.ID, "size", b.Size)
		if err := b.Load(); err != nil {
			return fmt.Errorf("load %s: %w", b.ID, err)
		}
		defer b.Close()
		log.Info("indexed backup", "id", b.ID,
			"resources", b.Index().Total(), "kinds", len(b.Index().Kinds()), "took", time.Since(start).Round(time.Millisecond))
	}

	mux := http.NewServeMux()
	api.New(backups, log).Routes(mux)
	if webui.Available() {
		mux.Handle("/", webui.Handler())
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "the web UI was not built into this binary, run: make ui", http.StatusNotImplemented)
		})
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           logRequests(mux, log),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + ln.Addr().String()
	fmt.Printf("\n  etcd Backup Browser is ready on %s\n\n", url)
	if *open {
		openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("stopped")
	return nil
}

func logRequests(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Debug("request", "method", r.Method, "path", r.URL.RequestURI(), "took", time.Since(start).Round(time.Millisecond))
		}
	})
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "explorer"
	default:
		cmd = "xdg-open"
	}
	exec.Command(cmd, url).Start()
}

// ---- info ----

func runInfo(args []string) error {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	path := backupFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := loadOne(*path)
	if err != nil {
		return err
	}
	defer b.Close()

	idx := b.Index()
	i := idx.Info
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "Snapshot\t%s\n", b.SnapshotPath)
	fmt.Fprintf(w, "Static resources\t%s\n", orNone(b.StaticPath))
	fmt.Fprintf(w, "Taken at\t%s\n", b.TakenAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Size\t%.1f MiB\n", float64(i.Size)/(1<<20))
	fmt.Fprintf(w, "etcd version\t%s\n", i.ClusterVersion)
	fmt.Fprintf(w, "Revision\t%d\n", i.Revision)
	fmt.Fprintf(w, "Compact revision\t%d\n", i.CompactRev)
	fmt.Fprintf(w, "Consistent index\t%d\n", i.ConsistentIdx)
	fmt.Fprintf(w, "Keys stored\t%d\n", i.TotalKeys)
	fmt.Fprintf(w, "Live keys\t%d\n", i.LiveKeys)
	fmt.Fprintf(w, "Tombstones\t%d\n", i.Tombstones)
	fmt.Fprintf(w, "Leases\t%d\n", i.Leases)
	fmt.Fprintf(w, "Kubernetes objects\t%d\n", idx.Total())
	fmt.Fprintf(w, "Resource types\t%d\n", len(idx.Kinds()))
	fmt.Fprintf(w, "Namespaces\t%d\n", len(idx.Namespaces()))
	if a := b.Archive(); a != nil {
		fmt.Fprintf(w, "Static files\t%d\n", len(a.Files))
	}
	fmt.Fprintf(w, "Indexed in\t%s\n", idx.BuiltIn.Round(time.Millisecond))
	w.Flush()

	fmt.Println("\nMembers")
	w = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "  ID\tNAME\tPEER URLS")
	for _, m := range i.Members {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", m.ID, m.Name, strings.Join(m.PeerURLs, ","))
	}
	return w.Flush()
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// ---- kinds ----

func runKinds(args []string) error {
	fs := flag.NewFlagSet("kinds", flag.ExitOnError)
	path := backupFlag(fs)
	byCount := fs.Bool("by-count", false, "order by object count instead of the console order")
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := loadOne(*path)
	if err != nil {
		return err
	}
	defer b.Close()

	kinds := b.Index().Kinds()
	if *byCount {
		sort.SliceStable(kinds, func(i, j int) bool { return kinds[i].Count > kinds[j].Count })
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "COUNT\tKIND\tAPI VERSION\tSCOPE")
	for _, k := range kinds {
		scope := "Cluster"
		if k.Namespaced {
			scope = "Namespaced"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", k.Count, k.Kind, k.APIVersion, scope)
	}
	return w.Flush()
}

// ---- ls ----

func runLs(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	path := backupFlag(fs)
	kind := fs.String("kind", "", `resource type, either a Kind such as "Pod" or a full id such as "v1/Pod"`)
	namespace := fs.String("namespace", "", "restrict to one namespace")
	q := fs.String("q", "", "match name, namespace or kind")
	deep := fs.Bool("deep", false, "also search inside the stored object bodies")
	limit := fs.Int("limit", 100, "maximum rows to print, 0 for all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	b, err := loadOne(*path)
	if err != nil {
		return err
	}
	defer b.Close()
	idx := b.Index()

	kindID, err := resolveKind(idx, *kind)
	if err != nil {
		return err
	}
	res, err := idx.List(index.Query{
		KindID: kindID, Namespace: *namespace, Search: *q, Deep: *deep,
		Sort: "name", Limit: *limit,
	})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tNAME\tKIND\tCREATED\tSIZE")
	for _, r := range res.Items {
		created := "-"
		if r.Created != nil {
			created = r.Created.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", orDash(r.Namespace), r.Name, r.Kind, created, r.Size)
	}
	w.Flush()
	fmt.Printf("\n%d of %d matching objects\n", len(res.Items), res.Total)
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ---- get ----

func runGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	path := backupFlag(fs)
	kind := fs.String("kind", "", "resource type, for example v1/ConfigMap or ConfigMap")
	namespace := fs.String("namespace", "", "namespace of the object")
	name := fs.String("name", "", "name of the object")
	format := fs.String("o", "yaml", "output format: yaml, json or raw")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("-name is required")
	}
	b, err := loadOne(*path)
	if err != nil {
		return err
	}
	defer b.Close()
	idx := b.Index()

	kindID, err := resolveKind(idx, *kind)
	if err != nil {
		return err
	}
	res, err := idx.List(index.Query{KindID: kindID, Namespace: *namespace, Search: *name, Sort: "name"})
	if err != nil {
		return err
	}
	var found *index.Resource
	for _, r := range res.Items {
		if r.Name == *name {
			found = r
			break
		}
	}
	if found == nil {
		return fmt.Errorf("no object named %q found (%d near matches)", *name, res.Total)
	}

	body, err := idx.Value(found)
	if err != nil {
		return err
	}
	if *format == "raw" {
		_, err := os.Stdout.Write(body)
		return err
	}
	obj, err := kube.Decode(body)
	if err != nil {
		return err
	}
	if obj.Reason != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", obj.Reason)
	}
	if *format == "json" {
		fmt.Println(string(obj.JSON))
		return nil
	}
	fmt.Print(obj.YAML)
	return nil
}

// resolveKind accepts a full kind id such as "apps/v1/Deployment" or a bare
// Kind such as "Deployment", and reports the ambiguity when a bare Kind exists
// in more than one API group.
func resolveKind(idx *index.Index, want string) (string, error) {
	if want == "" {
		return "", nil
	}
	var matches []string
	for _, k := range idx.Kinds() {
		if k.ID == want {
			return k.ID, nil
		}
		if strings.EqualFold(k.Kind, want) {
			matches = append(matches, k.ID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no resource type matches %q, try: openshift-etcd-backup-explorer kinds", want)
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("%q is ambiguous, it matches %s", want, strings.Join(matches, ", "))
}
