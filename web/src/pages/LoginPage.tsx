import { Link, Navigate, useNavigate } from "react-router-dom";

import { LoginForm } from "@/features/auth";
import { useAuth } from "@/features/auth/hooks/useAuth";

export default function LoginPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuth((s) => s.isAuthenticated);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div>
      <LoginForm onSuccess={() => navigate("/dashboard")} />
      <p style={{ textAlign: "center", fontSize: "0.875rem" }}>
        Don&apos;t have an account? <Link to="/register">Create one</Link>
      </p>
    </div>
  );
}
