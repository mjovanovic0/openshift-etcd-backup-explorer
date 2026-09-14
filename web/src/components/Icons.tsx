/** Small inline icon set, so the UI pulls in no icon dependency. */

type Props = { className?: string };

const base = "h-4 w-4 shrink-0";

export const IconFolder = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
    <path d="M1.5 3A1.5 1.5 0 0 1 3 1.5h3.1c.4 0 .78.16 1.06.44L8.5 3.5H13A1.5 1.5 0 0 1 14.5 5v7A1.5 1.5 0 0 1 13 13.5H3A1.5 1.5 0 0 1 1.5 12V3Zm1.5-.5a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 .5.5h10a.5.5 0 0 0 .5-.5V5a.5.5 0 0 0-.5-.5H8.09L6.45 2.85a.5.5 0 0 0-.35-.15H3Z" />
  </svg>
);

export const IconGlobe = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
    <circle cx="8" cy="8" r="6.4" />
    <path d="M1.6 8h12.8M8 1.6c1.7 1.8 2.6 4 2.6 6.4S9.7 12.6 8 14.4C6.3 12.6 5.4 10.4 5.4 8S6.3 3.4 8 1.6Z" />
  </svg>
);

export const IconGrid = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
    <path d="M2 2h5v5H2V2Zm7 0h5v5H9V2ZM2 9h5v5H2V9Zm7 0h5v5H9V9Z" />
  </svg>
);

export const IconSearch = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
    <circle cx="6.8" cy="6.8" r="4.6" />
    <path d="m10.4 10.4 3.4 3.4" strokeLinecap="round" />
  </svg>
);

export const IconInfo = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
    <circle cx="8" cy="8" r="6.4" />
    <path d="M8 7v4.2" strokeLinecap="round" />
    <circle cx="8" cy="4.9" r=".85" fill="currentColor" stroke="none" />
  </svg>
);

export const IconGear = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
    <circle cx="8" cy="8" r="2.3" />
    <path d="M8 1.4v1.7M8 12.9v1.7M2.9 2.9l1.2 1.2M11.9 11.9l1.2 1.2M1.4 8h1.7M12.9 8h1.7M2.9 13.1l1.2-1.2M11.9 4.1l1.2-1.2" strokeLinecap="round" />
  </svg>
);

export const IconCube = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
    <path d="M8 1.6 14 4.8v6.4L8 14.4 2 11.2V4.8L8 1.6Z" strokeLinejoin="round" />
    <path d="M2 4.8 8 8m0 0 6-3.2M8 8v6.4" strokeLinejoin="round" />
  </svg>
);

export const IconList = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
    <path d="M2 3.2h2v1.6H2V3.2Zm3.6 0H14v1.6H5.6V3.2ZM2 7.2h2v1.6H2V7.2Zm3.6 0H14v1.6H5.6V7.2ZM2 11.2h2v1.6H2v-1.6Zm3.6 0H14v1.6H5.6v-1.6Z" />
  </svg>
);

export const IconDownload = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" aria-hidden="true">
    <path d="M8 2v7.2M5.2 6.6 8 9.4l2.8-2.8M2.8 12.6h10.4" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

export const IconCopy = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
    <rect x="5.6" y="5.6" width="8" height="8.8" rx="1.2" />
    <path d="M10.4 5.6V2.8a1.2 1.2 0 0 0-1.2-1.2H3.6a1.2 1.2 0 0 0-1.2 1.2v5.6a1.2 1.2 0 0 0 1.2 1.2h2" />
  </svg>
);

export const IconChevron = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
    <path d="m4 6.4 4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

export const IconArrowLeft = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
    <path d="M9.6 3.6 5.2 8l4.4 4.4" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

export const IconArrowRight = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
    <path d="M6.4 3.6 10.8 8l-4.4 4.4" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

export const IconFile = ({ className = base }: Props) => (
  <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" aria-hidden="true">
    <path d="M9 1.8H4.4A1.4 1.4 0 0 0 3 3.2v9.6a1.4 1.4 0 0 0 1.4 1.4h7.2a1.4 1.4 0 0 0 1.4-1.4V5.8L9 1.8Z" strokeLinejoin="round" />
    <path d="M9 1.8v4h4" strokeLinejoin="round" />
  </svg>
);

/** The Red Hat hat mark, drawn simply so the header needs no image asset. */
export const RedHatMark = ({ className = "h-7 w-7" }: Props) => (
  <svg className={className} viewBox="0 0 32 32" aria-hidden="true">
    <circle cx="16" cy="16" r="16" fill="#ee0000" />
    <path
      fill="#fff"
      d="M17.8 18.6c1.8 0 4.3-.37 4.3-2.5a2 2 0 0 0-.04-.49l-1.05-4.55c-.24-1-.45-1.46-2.2-2.34-1.36-.69-4.3-1.83-5.18-1.83-.81 0-1.05 1.05-2.02 1.05-.94 0-1.63-.78-2.5-.78-.84 0-1.39.57-1.81 1.75 0 0-1.18 3.33-1.33 4.06a.96.96 0 0 0-.02.28c0 1.94 7.63 5.35 11.87 5.35Zm4.57-1.6c.25 1.18.25 1.3.25 1.46 0 2.02-2.27 3.14-5.26 3.14-6.75 0-12.66-3.95-12.66-6.56 0-.36.07-.72.2-1.06-2.2.11-5.05.5-5.05 3.02 0 4.13 9.78 9.22 17.52 9.22 5.94 0 7.43-2.69 7.43-4.81 0-1.67-1.44-3.56-2.43-4.4Z"
    />
  </svg>
);
