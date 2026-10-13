import { Link } from "react-router-dom";

const FEATURES = [
  { title: "Boards", text: "Kanban boards with drag & drop that follow your workflow's rules." },
  { title: "Backlog & sprints", text: "Rank the backlog, plan sprints with story points, start and complete them." },
  { title: "Reports", text: "Burndown, velocity, created vs resolved, cycle time and workload at a glance." },
  { title: "Permissions", text: "Admin, member and viewer roles per project, with Jira-style permission schemes on the way." },
  { title: "Import from Jira", text: "Bring your Jira CSV export over with a preview — and export back any time." },
  { title: "Self-hosted", text: "One Go binary and PostgreSQL. Your data stays on your servers. MIT licensed." },
];

/** Public home page for visitors (KYB-S36). */
export function LandingPage() {
  return (
    <div className="landing">
      <header className="landing-nav">
        <Link to="/" className="brand">
          <img src="/logo.svg" alt="" width={28} height={28} /> Kyber
        </Link>
        <nav aria-label="Account">
          <Link to="/login">Log in</Link>
          <Link to="/signup" className="button">
            Start free
          </Link>
        </nav>
      </header>
      <main>
        <section className="hero">
          <img src="/logo.svg" alt="" width={140} height={140} className="hero-logo" />
          <h1>Plan, track and ship software — on your own terms.</h1>
          <p className="lead">
            Kyber is an open-source issue and project tracker with the Jira essentials: boards, backlog, sprints, reports
            and permissions. Use it here or run it on your own servers.
          </p>
          <div className="cta">
            <Link to="/signup" className="button">
              Start free
            </Link>
            <a href="https://github.com/bakhod1r/kyber" className="button ghost">
              View on GitHub
            </a>
          </div>
        </section>
        <section aria-labelledby="features-title" className="features" role="region" aria-label="Features">
          <h2 id="features-title" className="visually-hidden">
            Features
          </h2>
          <ul>
            {FEATURES.map((f) => (
              <li key={f.title}>
                <h3>{f.title}</h3>
                <p>{f.text}</p>
              </li>
            ))}
          </ul>
        </section>
        <section className="selfhost" aria-labelledby="selfhost-title">
          <h2 id="selfhost-title">Run it yourself in a minute</h2>
          <pre>
            <code>docker compose up -d   # Kyber + PostgreSQL on http://localhost:8080</code>
          </pre>
        </section>
      </main>
      <footer className="landing-foot">
        <span>© Kyber contributors · MIT License</span>
        <Link to="/signup">Start free</Link>
      </footer>
    </div>
  );
}
