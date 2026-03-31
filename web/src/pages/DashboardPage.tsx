import { ShortenForm, LinkList } from "@/features/links";

import styles from "./DashboardPage.module.css";

export default function DashboardPage() {
  return (
    <div>
      <h1 className={styles.title}>Dashboard</h1>
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Create a new link</h2>
        <ShortenForm />
      </section>
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Your links</h2>
        <LinkList />
      </section>
    </div>
  );
}
