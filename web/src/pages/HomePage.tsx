import { ShortenForm } from "@/features/links";

import styles from "./HomePage.module.css";

export default function HomePage() {
  return (
    <div className={styles.container}>
      <h1 className={styles.title}>Shorten any URL</h1>
      <p className={styles.subtitle}>Paste a long URL and get a short one instantly.</p>
      <ShortenForm />
    </div>
  );
}
