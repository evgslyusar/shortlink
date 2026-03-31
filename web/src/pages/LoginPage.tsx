import { Link, Navigate, useNavigate } from "react-router-dom";

import { LoginForm, useAuth } from "@/features/auth";

import styles from "./AuthPage.module.css";

export default function LoginPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuth((s) => s.isAuthenticated);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div>
      <LoginForm onSuccess={() => navigate("/dashboard")} />
      <p className={styles.footer}>
        Don&apos;t have an account? <Link to="/register">Create one</Link>
      </p>
    </div>
  );
}
