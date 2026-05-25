import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getJSON, postJSON } from "../lib/api";
import type { AuthStatus } from "../lib/types";

export function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("请输入后台管理员账号密码。");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    void getJSON<AuthStatus>("/api/auth/me")
      .then((status) => {
        if (status.authenticated) {
          navigate("/");
        }
      })
      .catch(() => {});
  }, [navigate]);

  async function handleLogin() {
    setLoading(true);
    try {
      await postJSON("/api/auth/login", { username, password });
      navigate("/");
    } catch {
      setMessage("登录失败，请检查账号和密码。");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-header">
          <div className="login-logo">S</div>
          <h1>SubForMe</h1>
          <p>Mihomo 配置管理系统</p>
        </div>
        <div className="login-body">
          <div className="form-group">
            <label>用户名</label>
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              onKeyDown={(event) => event.key === "Enter" && !loading && void handleLogin()}
            />
          </div>
          <div className="form-group">
            <label>密码</label>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              onKeyDown={(event) => event.key === "Enter" && !loading && void handleLogin()}
            />
          </div>
          <button className="btn btn-primary login-submit" type="button" onClick={() => void handleLogin()} disabled={loading}>
            {loading ? "登录中..." : "登录"}
          </button>
          <p className="login-message">{message}</p>
        </div>
      </div>
    </div>
  );
}
