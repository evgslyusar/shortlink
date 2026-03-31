import { Button } from "@/shared/components/Button";
import type { Link } from "../types";
import styles from "./LinkList.module.css";

interface LinkRowProps {
  link: Link;
  onDelete: (slug: string) => void;
  onViewStats: (slug: string) => void;
  isDeleting: boolean;
}

export function LinkRow({ link, onDelete, onViewStats, isDeleting }: LinkRowProps) {
  return (
    <tr className={styles.row}>
      <td>
        <a href={link.short_url} target="_blank" rel="noopener noreferrer">
          {link.slug}
        </a>
      </td>
      <td className={styles.urlCell} title={link.original_url}>
        {link.original_url}
      </td>
      <td>{new Date(link.created_at).toLocaleDateString()}</td>
      <td>
        <div className={styles.actions}>
          <Button variant="ghost" onClick={() => onViewStats(link.slug)}>
            Stats
          </Button>
          <Button variant="danger" onClick={() => onDelete(link.slug)} disabled={isDeleting}>
            Delete
          </Button>
        </div>
      </td>
    </tr>
  );
}
