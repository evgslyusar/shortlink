import { useState } from "react";

import { useLinks } from "../hooks/useLinks";
import { useDeleteLink } from "../hooks/useDeleteLink";
import { LinkRow } from "./LinkRow";
import { LinkStats } from "./LinkStats";
import { Button } from "@/shared/components/Button";
import styles from "./LinkList.module.css";

export function LinkList() {
  const [page, setPage] = useState(1);
  const [statsSlug, setStatsSlug] = useState<string | null>(null);
  const { data, isLoading, error } = useLinks(page);
  const deleteMutation = useDeleteLink();

  if (isLoading) return <p>Loading links...</p>;
  if (error) return <p>Failed to load links.</p>;

  const links = data?.data.items ?? [];
  const total = data?.meta.total ?? 0;
  const perPage = data?.meta.per_page ?? 20;
  const totalPages = Math.ceil(total / perPage);

  return (
    <div>
      {links.length === 0 ? (
        <p className={styles.empty}>No links yet. Create one above.</p>
      ) : (
        <>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Slug</th>
                <th>Original URL</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {links.map((link) => (
                <LinkRow
                  key={link.slug}
                  link={link}
                  onDelete={(slug) => deleteMutation.mutate(slug)}
                  onViewStats={(slug) => setStatsSlug(slug === statsSlug ? null : slug)}
                  isDeleting={deleteMutation.isPending}
                />
              ))}
            </tbody>
          </table>
          {totalPages > 1 && (
            <div className={styles.pagination}>
              <Button variant="ghost" onClick={() => setPage((p) => p - 1)} disabled={page <= 1}>
                Previous
              </Button>
              <span>
                Page {page} of {totalPages}
              </span>
              <Button
                variant="ghost"
                onClick={() => setPage((p) => p + 1)}
                disabled={page >= totalPages}
              >
                Next
              </Button>
            </div>
          )}
        </>
      )}
      {statsSlug && <LinkStats slug={statsSlug} onClose={() => setStatsSlug(null)} />}
    </div>
  );
}
