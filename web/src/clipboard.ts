/**
 * copyText puts text on the clipboard. The async clipboard API needs a secure
 * context, which 127.0.0.1 is, but a browser reached over a plain address on
 * the network is not, so there is a fallback for that case.
 */
export async function copyText(text: string): Promise<void> {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.top = "-1000px";
  area.style.opacity = "0";
  document.body.appendChild(area);
  try {
    area.select();
    if (!document.execCommand("copy")) {
      throw new Error("the browser refused the copy");
    }
  } finally {
    document.body.removeChild(area);
  }
}
