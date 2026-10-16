import { Link } from "react-router-dom";
import { msg, type MessageKey, useT } from "../shared/i18n/i18n";

const FEATURES: { title: MessageKey; text: MessageKey }[] = [
  { title: msg("Boards"), text: msg("Kanban boards with drag & drop that follow your workflow's rules.") },
  { title: msg("Backlog & sprints"), text: msg("Rank the backlog, plan sprints with story points, start and complete them.") },
  { title: msg("Reports"), text: msg("Burndown, velocity, created vs resolved, cycle time and workload at a glance.") },
  { title: msg("Permissions"), text: msg("Admin, member and viewer roles per project, with Jira-style permission schemes on the way.") },
  { title: msg("Import from Jira"), text: msg("Bring your Jira CSV export over with a preview — and export back any time.") },
  { title: msg("Self-hosted"), text: msg("One Go binary and PostgreSQL. Your data stays on your servers. MIT licensed.") },
];

/** Public home page for visitors (KYB-S36). */
export function LandingPage() {
  const t = useT();
  return (
    <div className="landing">
      <header className="landing-nav">
        <Link to="/" className="brand">
          <img src={`${import.meta.env.BASE_URL}logo.svg`} alt="" width={28} height={28} /> Kyber
        </Link>
        <nav aria-label={t("Account")}>
          <Link to="/login">{t("Log in")}</Link>
          <Link to="/signup" className="button">
            {t("Start free")}
          </Link>
        </nav>
      </header>
      <main>
        <section className="hero">
          <img src={`${import.meta.env.BASE_URL}logo.svg`} alt="" width={140} height={140} className="hero-logo" />
          <h1>{t("Plan, track and ship software — on your own terms.")}</h1>
          <p className="lead">
            {t("Kyber is an open-source issue and project tracker with the Jira essentials: boards, backlog, sprints, reports and permissions. Use it here or run it on your own servers.")}
          </p>
          <div className="cta">
            <Link to="/signup" className="button">
              {t("Start free")}
            </Link>
            <a href="https://github.com/bakhod1r/kyber" className="button ghost">
              {t("View on GitHub")}
            </a>
          </div>
        </section>
        <section aria-labelledby="features-title" className="features" role="region" aria-label={t("Features")}>
          <h2 id="features-title" className="visually-hidden">
            {t("Features")}
          </h2>
          <ul>
            {FEATURES.map((f) => (
              <li key={f.title}>
                <h3>{t(f.title)}</h3>
                <p>{t(f.text)}</p>
              </li>
            ))}
          </ul>
        </section>
        <section className="selfhost" aria-labelledby="selfhost-title">
          <h2 id="selfhost-title">{t("Run it yourself in a minute")}</h2>
          <pre>
            <code>{t("docker compose up -d   # Kyber + PostgreSQL on http://localhost:8080")}</code>
          </pre>
        </section>
      </main>
      <footer className="landing-foot">
        <span>{t("© Kyber contributors · MIT License")}</span>
        <Link to="/signup">{t("Start free")}</Link>
      </footer>
    </div>
  );
}
