import { type FormEvent, useState } from "react";

import { Button } from "@/shared/components/Button";
import { Input } from "@/shared/components/Input";
import { ApiClientError } from "@/api/client";

import { useCreateLink } from "../hooks/useCreateLink";
import type { CreateLinkRequest, CreateLinkResponse } from "../types";

import styles from "./ShortenForm.module.css";

interface ShortenFormProps {
  onCreated?: (link: CreateLinkResponse) => void;
}

export function ShortenForm({ onCreated }: ShortenFormProps) {
  const [url, setUrl] = useState("");
  const [slug, setSlug] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [result, setResult] = useState<CreateLinkResponse | null>(null);
  const createMutation = useCreateLink();

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    setResult(null);

    const payload: CreateLinkRequest = { url };
    if (slug.trim()) {
      payload.slug = slug.trim();
    }
    if (expiresAt) {
      payload.expires_at = new Date(expiresAt).toISOString();
    }

    createMutation.mutate(payload, {
      onSuccess: (res) => {
        setResult(res.data);
        setUrl("");
        setSlug("");
        setExpiresAt("");
        onCreated?.(res.data);
      },
    });
  };

  const errorMessage =
    createMutation.error instanceof ApiClientError ? createMutation.error.message : undefined;

  return (
    <div>
      <form onSubmit={handleSubmit} className={styles.form}>
        <Input
          label="URL to shorten"
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
          placeholder="https://example.com/very-long-url"
        />
        <Input
          label="Custom slug (optional)"
          type="text"
          value={slug}
          onChange={(e) => setSlug(e.target.value)}
          placeholder="my-custom-slug"
        />
        <Input
          label="Expiry date (optional)"
          type="datetime-local"
          value={expiresAt}
          onChange={(e) => setExpiresAt(e.target.value)}
        />
        {errorMessage && (
          <p className={styles.error} role="alert">
            {errorMessage}
          </p>
        )}
        <Button type="submit" disabled={createMutation.isPending}>
          {createMutation.isPending ? "Shortening..." : "Shorten"}
        </Button>
      </form>
      {result && (
        <div className={styles.result}>
          <p>Short URL:</p>
          <a href={result.short_url} target="_blank" rel="noopener noreferrer">
            {result.short_url}
          </a>
        </div>
      )}
    </div>
  );
}
