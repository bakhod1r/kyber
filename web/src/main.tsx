import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, HashRouter } from "react-router-dom";
import { AppRoutes } from "./app/App";
import { DemoBanner } from "./app/demo/DemoBanner";
import { installDemo } from "./app/demo/install";
import { applyAppearance, loadAppearance } from "./shared/appearance";
import { I18nProvider } from "./shared/i18n/i18n";
import "./styles.css";

/** The GitHub Pages demo (VITE_DEMO=1) answers the API in the browser; hash routes need no server fallback. */
const demo = import.meta.env.VITE_DEMO === "1";
if (demo) installDemo();
const Router = demo ? HashRouter : BrowserRouter;

applyAppearance(loadAppearance());

const client = new QueryClient({ defaultOptions: { queries: { retry: demo ? 0 : 1, refetchOnWindowFocus: !demo } } });

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <I18nProvider>
      <QueryClientProvider client={client}>
        <Router>
          {demo && <DemoBanner />}
          <AppRoutes />
        </Router>
      </QueryClientProvider>
    </I18nProvider>
  </StrictMode>,
);
