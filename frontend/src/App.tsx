import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { DashboardPage } from "./pages/DashboardPage";
import { GroupsPage } from "./pages/GroupsPage";
import { LoginPage } from "./pages/LoginPage";
import { NodesPage } from "./pages/NodesPage";
import { TemplatesPage } from "./pages/TemplatesPage";
import { UserPreviewPage } from "./pages/UserPreviewPage";
import { XuiSettingsPage } from "./pages/XuiSettingsPage";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<Layout />}>
          <Route index element={<DashboardPage />} />
          <Route path="settings" element={<XuiSettingsPage />} />
          <Route path="users" element={<UserPreviewPage />} />
          <Route path="nodes" element={<NodesPage />} />
          <Route path="groups" element={<GroupsPage />} />
          <Route path="templates" element={<TemplatesPage />} />
          <Route path="xui" element={<Navigate to="/settings" replace />} />
          <Route path="preview" element={<Navigate to="/users" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
