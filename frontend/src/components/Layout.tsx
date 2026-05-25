import { useEffect, useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { getJSON, postJSON } from "../lib/api";
import type { AuthStatus } from "../lib/types";

const navItems = [
  { to: "/", label: "概览", end: true },
  { to: "/settings", label: "设置" },
  { to: "/users", label: "用户" },
  { to: "/nodes", label: "节点" },
  { to: "/servers", label: "服务器" },
  { to: "/groups", label: "代理分组" },
  { to: "/providers", label: "第三方 Providers" },
  { to: "/templates", label: "模板" },
];

type ThemeMode = "light" | "dark";

function initialTheme(): ThemeMode {
  const saved = localStorage.getItem("subforme-theme");
  if (saved === "light" || saved === "dark") {
    return saved;
  }
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function Layout() {
  const navigate = useNavigate();
  const [authChecked, setAuthChecked] = useState(false);
  const [version, setVersion] = useState("");
  const [theme, setTheme] = useState<ThemeMode>(() => initialTheme());

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("subforme-theme", theme);
  }, [theme]);

  useEffect(() => {
    void getJSON<AuthStatus>("/api/auth/me")
      .then((status) => {
        if (!status.authenticated) {
          navigate("/login");
          return;
        }
        setAuthChecked(true);
      })
      .catch(() => {
        navigate("/login");
      });
    void getJSON<{ version: string }>("/api/version")
      .then((v) => setVersion(v.version))
      .catch(() => {});
  }, [navigate]);

  async function handleLogout() {
    try {
      await postJSON("/api/auth/logout");
    } finally {
      navigate("/login");
    }
  }

  if (!authChecked) {
    return (
      <div className="loading-screen">
        <div className="loading-box">正在检查登录状态...</div>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-logo">
            <div className="logo-icon">S</div>
            <span>SubForMe</span>
          </div>
        </div>
        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end}>
              <span className="nav-label">{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-footer">
          <button className="sidebar-theme" type="button" onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
            <span className="nav-label">{theme === "dark" ? "浅色模式" : "深色模式"}</span>
          </button>
          <div className="sidebar-version">{version}</div>
          <button className="sidebar-logout" type="button" onClick={() => void handleLogout()}>
            <span className="nav-label">退出</span>
          </button>
        </div>
      </aside>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
