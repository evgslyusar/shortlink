import { type FormEvent, useState } from "react";

import { Button } from "@/shared/components/Button";
import { Input } from "@/shared/components/Input";
import { useRegister } from "../hooks/useRegister";
import { ApiClientError } from "@/api/client";
import styles from "./AuthForm.module.css";

export function RegisterForm({ onSuccess }: { onSuccess: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [clientError, setClientError] = useState("");
  const registerMutation = useRegister();

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    setClientError("");

    if (password !== confirmPassword) {
      setClientError("Passwords do not match");
      return;
    }

    registerMutation.mutate({ email, password }, { onSuccess });
  };

  const serverError =
    registerMutation.error instanceof ApiClientError ? registerMutation.error.message : undefined;

  const errorMessage = clientError || serverError;

  return (
    <form onSubmit={handleSubmit} className={styles.form}>
      <h1 className={styles.title}>Create account</h1>
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
        minLength={8}
        autoComplete="new-password"
      />
      <Input
        label="Confirm password"
        type="password"
        value={confirmPassword}
        onChange={(e) => setConfirmPassword(e.target.value)}
        required
        minLength={8}
        autoComplete="new-password"
      />
      <Button type="submit" disabled={registerMutation.isPending}>
        {registerMutation.isPending ? "Creating account..." : "Create account"}
      </Button>
    </form>
  );
}
