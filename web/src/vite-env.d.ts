/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "1" builds the GitHub Pages demo: the API is answered in the browser (src/app/demo). */
  readonly VITE_DEMO?: string;
}
