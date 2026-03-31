import { Link, Navigate, useNavigate } from "react-router-dom";

import { RegisterForm } from "@/features/auth";
import { useAuth } from "@/features/auth/hooks/useAuth";

export default function RegisterPage() {
  const navigate = useNavigate();
  const isAuthenticated = useAuth((s) => s.isAuthenticated);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div>
      <RegisterForm onSuccess={() => navigate("/login")} />
      <p style={{ textAlign: "center", fontSize: "0.875rem" }}>
        Already have an account? <Link to="/login">Sign in</Link>
      </p>
    </div>
  );
}
