import { Link, Outlet } from "react-router-dom";

import { useAuth } from "@/features/auth/hooks/useAuth";
import { useLogout } from "@/features/auth/hooks/useLogout";
import { Button } from "./Button";
import styles from "./Layout.module.css";

export function Layout() {
  const { isAuthenticated, user, clearAuth } = useAuth();
  const logoutMutation = useLogout();

  const handleLogout = () => {
    logoutMutation.mutate(undefined, {
      onSettled: () => clearAuth(),
    });
  };

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <nav className={styles.nav}>
          <Link to="/" className={styles.logo}>
            Slink
          </Link>
          <div className={styles.navLinks}>
            {isAuthenticated ? (
              <>
                <Link to="/dashboard" className={styles.link}>
                  Dashboard
                </Link>
                <span className={styles.email}>{user?.email}</span>
                <Button variant="ghost" onClick={handleLogout} disabled={logoutMutation.isPending}>
                  Logout
                </Button>
              </>
            ) : (
              <>
                <Link to="/login" className={styles.link}>
                  Login
                </Link>
                <Link to="/register" className={styles.link}>
                  Register
                </Link>
              </>
            )}
          </div>
        </nav>
      </header>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  );
}
