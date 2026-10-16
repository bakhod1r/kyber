import { useT } from "../../shared/i18n/i18n";
import { resetDemo } from "./install";

const REPO = "https://github.com/bakhod1r/kyber";

/** DemoBanner tells visitors of the Pages demo that their changes stay in this tab. */
export function DemoBanner() {
  const t = useT();
  return (
    <p className="demo-banner" role="note">
      {t("Demo: everything runs in your browser and is saved only here.")}{" "}
      <button type="button" className="link" onClick={resetDemo}>
        {t("Reset demo")}
      </button>
      {" · "}
      <a href={`${REPO}#quick-start`} target="_blank" rel="noreferrer">
        {t("Run Kyber yourself")}
      </a>
    </p>
  );
}
