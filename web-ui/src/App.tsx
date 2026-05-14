import { Routes, Route, Navigate } from "react-router";
import AppShell from "@/components/layout/app-shell";
import LoginPage from "@/pages/login";
import DashboardPage from "@/pages/dashboard";
import SubscriptionsPage from "@/pages/subscriptions";
import NodesPage from "@/pages/nodes";
import RuleSourcesPage from "@/pages/rule-sources";
import TemplatesPage from "@/pages/templates";
import ConfigsPage from "@/pages/configs";
import AccessLogsPage from "@/pages/access-logs";
import BackupPage from "@/pages/backup";
import ConverterPage from "@/pages/converter";

export default function App() {
  return (
    <Routes>
      {/* Public routes — no layout */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/converter" element={<ConverterPage />} />

      {/* Authenticated routes — wrapped in AppShell */}
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/subscriptions" element={<SubscriptionsPage />} />
        <Route path="/nodes" element={<NodesPage />} />
        <Route path="/rule-sources" element={<RuleSourcesPage />} />
        <Route path="/templates" element={<TemplatesPage />} />
        <Route path="/configs" element={<ConfigsPage />} />
        <Route path="/config-access-logs" element={<AccessLogsPage />} />
        <Route path="/backup" element={<BackupPage />} />
      </Route>
    </Routes>
  );
}
