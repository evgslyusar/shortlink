import { useLinkStats } from "../hooks/useLinkStats";
import { Button } from "@/shared/components/Button";
import styles from "./LinkStats.module.css";

interface LinkStatsProps {
  slug: string;
  onClose: () => void;
}

export function LinkStats({ slug, onClose }: LinkStatsProps) {
  const { data, isLoading, error } = useLinkStats(slug);

  if (isLoading) return <p>Loading stats...</p>;
  if (error) return <p>Failed to load stats.</p>;

  const stats = data?.data;
  if (!stats) return null;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h3>
          Stats for <code>{slug}</code>
        </h3>
        <Button variant="ghost" onClick={onClose}>
          Close
        </Button>
      </div>
      <p className={styles.total}>Total clicks: {stats.total_clicks}</p>
      {stats.by_day.length > 0 && (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Date</th>
              <th>Clicks</th>
            </tr>
          </thead>
          <tbody>
            {stats.by_day.map((day) => (
              <tr key={day.date}>
                <td>{day.date}</td>
                <td>{day.count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {stats.by_day.length === 0 && <p className={styles.empty}>No click data yet.</p>}
    </div>
  );
}
