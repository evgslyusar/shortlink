import { type FormEvent, useState } from "react";

import { Button } from "@/shared/components/Button";
import { Input } from "@/shared/components/Input";
import { useLogin } from "../hooks/useLogin";
import { useAuth } from "../hooks/useAuth";
import { ApiClientError } from "@/api/client";
import styles from "./AuthForm.module.css";

export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const loginMutation = useLogin();
  const setAuth = useAuth((s) => s.setAuth);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    loginMutation.mutate(
      { email, password },
      {
        onSuccess: (res) => {
          setAuth({ id: "", email });
          void res;
          onSuccess();
        },
      },
    );
  };

  const errorMessage =
    loginMutation.error instanceof ApiClientError ? loginMutation.error.message : undefined;

  return (
    <form onSubmit={handleSubmit} className={styles.form}>
      <h1 className={styles.title}>Sign in</h1>
      {errorMessage && (
        <p className={styles.error} role="alert">
          {errorMessage}
        </p>
      )}
      <Input
        label="Email"
        type="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        autoComplete="email"
      />
      <Input
        label="Password"
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        autoComplete="current-password"
      />
      <Button type="submit" disabled={loginMutation.isPending}>
        {loginMutation.isPending ? "Signing in..." : "Sign in"}
      </Button>
    </form>
  );
}
