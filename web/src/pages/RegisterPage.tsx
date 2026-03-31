import { Link, Navigate, useNavigate } from "react-router-dom";

import { RegisterForm, useAuth } from "@/features/auth";

import styles from "./AuthPage.module.css";

export default function RegisterPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuth((s) => s.isAuthenticated);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div>
      <RegisterForm onSuccess={() => navigate("/login")} />
      <p className={styles.footer}>
        Already have an account? <Link to="/login">Sign in</Link>
      </p>
    </div>
  );
}
