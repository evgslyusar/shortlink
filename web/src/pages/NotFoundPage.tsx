import { Link } from "react-router-dom";

export default function NotFoundPage() {
  return (
    <div style={{ textAlign: "center", padding: "4rem 0" }}>
      <h1 style={{ fontSize: "3rem", fontWeight: 700, color: "#111827" }}>404</h1>
      <p style={{ color: "#6b7280", marginBottom: "1.5rem" }}>Page not found.</p>
      <Link to="/" style={{ color: "#2563eb" }}>
        Go home
      </Link>
    </div>
  );
}
