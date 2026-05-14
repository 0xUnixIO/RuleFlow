import { useMemo } from "react";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { get } from "@/lib/api";
import type { Subscription, NodeStats, ConfigPolicy, Template } from "@/types";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  Rss,
  Server,
  Shield,
  FileCode2,
  ArrowRight,
  Activity,
  AlertCircle,
  Clock,
  Wifi,
  CalendarClock,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function relativeTime(iso: string | null): string {
  if (!iso) return "Never";
  const diff = Date.now() - new Date(iso).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return "Just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / 1024 ** i).toFixed(i > 1 ? 1 : 0)} ${units[i]}`;
}

function subscriptionHealth(
  sub: Subscription,
): { label: string; color: string; dotClass: string } {
  if (sub.last_fetch_error) {
    return { label: "Error", color: "text-red-400", dotClass: "bg-red-400" };
  }
  if (sub.last_fetched_at) {
    const age = Date.now() - new Date(sub.last_fetched_at).getTime();
    if (age > 24 * 60 * 60 * 1000) {
      return { label: "Stale", color: "text-yellow-400", dotClass: "bg-yellow-400" };
    }
  }
  return { label: "Normal", color: "text-emerald-400", dotClass: "bg-emerald-400" };
}

const PROTOCOL_COLORS: Record<string, string> = {
  trojan: "bg-blue-500",
  vmess: "bg-violet-500",
  vless: "bg-indigo-500",
  ss: "bg-emerald-500",
  ssr: "bg-teal-500",
  wireguard: "bg-orange-500",
  hysteria: "bg-pink-500",
  hysteria2: "bg-rose-500",
  tuic: "bg-amber-500",
};

function protocolColor(proto: string): string {
  return PROTOCOL_COLORS[proto.toLowerCase()] ?? "bg-muted-foreground";
}

// ---------------------------------------------------------------------------
// Skeleton loaders
// ---------------------------------------------------------------------------

function StatsCardSkeleton() {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <Skeleton className="h-4 w-24" />
          <Skeleton className="size-8 rounded-lg" />
        </div>
      </CardHeader>
      <CardContent>
        <Skeleton className="h-7 w-16" />
        <Skeleton className="mt-1.5 h-3.5 w-32" />
      </CardContent>
    </Card>
  );
}

function SubscriptionCardSkeleton() {
  return (
    <Card>
      <CardContent className="space-y-3 pt-1">
        <div className="flex items-center justify-between">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
        <Skeleton className="h-3.5 w-full" />
        <div className="flex gap-3">
          <Skeleton className="h-3.5 w-20" />
          <Skeleton className="h-3.5 w-20" />
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatsCard({
  title,
  icon: Icon,
  value,
  subtitle,
}: {
  title: string;
  icon: React.ElementType;
  value: string;
  subtitle: string;
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardDescription className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {title}
          </CardDescription>
          <div className="flex size-8 items-center justify-center rounded-lg bg-muted">
            <Icon className="size-4 text-muted-foreground" />
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="font-heading text-2xl font-bold tracking-tight">
          {value}
        </div>
        <p className="mt-0.5 text-xs text-muted-foreground">{subtitle}</p>
      </CardContent>
    </Card>
  );
}

function ProtocolBar({
  byProtocol,
  total,
}: {
  byProtocol: Record<string, number>;
  total: number;
}) {
  const sorted = useMemo(
    () => Object.entries(byProtocol).sort(([, a], [, b]) => b - a),
    [byProtocol],
  );

  if (sorted.length === 0) {
    return <p className="text-xs text-muted-foreground">No protocol data</p>;
  }

  return (
    <div className="space-y-2.5">
      {/* Segmented bar */}
      <div className="flex h-2.5 overflow-hidden rounded-full bg-muted">
        {sorted.map(([proto, count]) => (
          <div
            key={proto}
            className={`${protocolColor(proto)} transition-all`}
            style={{ width: `${total > 0 ? (count / total) * 100 : 0}%` }}
          />
        ))}
      </div>
      {/* Legend pills */}
      <div className="flex flex-wrap gap-1.5">
        {sorted.map(([proto, count]) => (
          <span
            key={proto}
            className="inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
          >
            <span className={`inline-block size-2 rounded-full ${protocolColor(proto)}`} />
            {proto}
            <span className="font-medium text-foreground">{count}</span>
          </span>
        ))}
      </div>
    </div>
  );
}

function TrafficBar({ sub }: { sub: Subscription }) {
  if (!sub.userinfo || sub.userinfo.total <= 0) return null;

  const { upload, download, total } = sub.userinfo;
  const used = upload + download;
  const pct = Math.min((used / total) * 100, 100);
  const high = pct > 80;

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>
          {formatBytes(used)} / {formatBytes(total)}
        </span>
        <span className={high ? "font-medium text-orange-400" : ""}>
          {pct.toFixed(1)}%
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted">
        <div
          className={`h-full rounded-full transition-all ${high ? "bg-orange-500" : "bg-emerald-500"}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function ExpiryInfo({ sub }: { sub: Subscription }) {
  if (!sub.userinfo?.expire) return null;

  const expireDate = new Date(sub.userinfo.expire * 1000);
  const now = Date.now();
  const diffMs = expireDate.getTime() - now;
  const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));
  const dateStr = expireDate.toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });

  const expired = diffDays <= 0;
  const soon = !expired && diffDays <= 7;

  return (
    <div
      className={cn(
        "flex items-center gap-1.5 text-xs",
        expired
          ? "font-medium text-red-400"
          : soon
            ? "text-orange-400"
            : "text-muted-foreground",
      )}
    >
      <CalendarClock className="size-3 shrink-0" />
      <span>
        {expired
          ? `已过期 (${dateStr})`
          : `${dateStr}，剩余 ${diffDays} 天`}
      </span>
    </div>
  );
}

function SubscriptionHealthCard({ sub }: { sub: Subscription }) {
  const health = subscriptionHealth(sub);

  return (
    <Card>
      <CardContent className="space-y-3 pt-1">
        {/* Header */}
        <div className="flex items-start justify-between gap-2">
          <h3 className="min-w-0 truncate font-heading text-sm font-semibold">
            {sub.name}
          </h3>
          <Badge
            variant={sub.enabled ? "default" : "secondary"}
            className="shrink-0 text-[0.65rem]"
          >
            {sub.enabled ? "Enabled" : "Disabled"}
          </Badge>
        </div>

        {/* Status row */}
        <div className="flex items-center gap-4 text-xs">
          <span className={`flex items-center gap-1.5 ${health.color}`}>
            <span className={`inline-block size-1.5 rounded-full ${health.dotClass}`} />
            {health.label}
          </span>
          <span className="flex items-center gap-1 text-muted-foreground">
            <Server className="size-3" />
            {sub.node_count} nodes
          </span>
          <span className="flex items-center gap-1 text-muted-foreground">
            <Clock className="size-3" />
            {relativeTime(sub.last_fetched_at)}
          </span>
        </div>

        {/* Error */}
        {sub.last_fetch_error && (
          <div className="flex items-start gap-1.5 rounded-md bg-red-500/10 px-2.5 py-1.5 text-xs text-red-400">
            <AlertCircle className="mt-0.5 size-3 shrink-0" />
            <span className="line-clamp-2">{sub.last_fetch_error}</span>
          </div>
        )}

        {/* Traffic */}
        <TrafficBar sub={sub} />

        {/* Expiry */}
        <ExpiryInfo sub={sub} />
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function DashboardPage() {
  const { data: subscriptions, isLoading: subsLoading } = useQuery({
    queryKey: ["subscriptions"],
    queryFn: () => get<Subscription[]>("/api/subscriptions"),
    refetchInterval: 30_000,
  });

  const { data: nodeStats, isLoading: nodesLoading } = useQuery({
    queryKey: ["nodes", "stats"],
    queryFn: () => get<NodeStats>("/api/nodes/stats"),
    refetchInterval: 30_000,
  });

  const { data: configPolicies, isLoading: policiesLoading } = useQuery({
    queryKey: ["config-policies"],
    queryFn: () => get<ConfigPolicy[]>("/api/config-policies"),
    refetchInterval: 30_000,
  });

  const { data: templates, isLoading: templatesLoading } = useQuery({
    queryKey: ["templates"],
    queryFn: () => get<Template[]>("/api/templates"),
    refetchInterval: 30_000,
  });

  const statsLoading = subsLoading || nodesLoading || policiesLoading || templatesLoading;

  const enabledSubs = subscriptions?.filter((s) => s.enabled).length ?? 0;
  const enabledPolicies = configPolicies?.filter((p) => p.enabled).length ?? 0;

  return (
    <div className="space-y-6 p-4 lg:p-6">
      {/* Page header */}
      <div>
        <h1 className="font-heading text-2xl font-bold tracking-tight text-foreground">
          Dashboard
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Overview of your RuleFlow instance
        </p>
      </div>

      {/* Stats cards */}
      {statsLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <StatsCardSkeleton key={i} />
          ))}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatsCard
            title="Subscriptions"
            icon={Rss}
            value={`${enabledSubs} / ${subscriptions?.length ?? 0}`}
            subtitle={`${enabledSubs} enabled`}
          />
          <StatsCard
            title="Nodes"
            icon={Server}
            value={`${nodeStats?.enabled ?? 0} / ${nodeStats?.total ?? 0}`}
            subtitle={`${nodeStats?.disabled ?? 0} disabled`}
          />
          <StatsCard
            title="Config Policies"
            icon={Shield}
            value={`${enabledPolicies} / ${configPolicies?.length ?? 0}`}
            subtitle={`${enabledPolicies} enabled`}
          />
          <StatsCard
            title="Templates"
            icon={FileCode2}
            value={String(templates?.length ?? 0)}
            subtitle="Total templates"
          />
        </div>
      )}

      {/* Protocol distribution */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <Activity className="size-4 text-muted-foreground" />
            Protocol Distribution
          </CardTitle>
        </CardHeader>
        <CardContent>
          {nodesLoading ? (
            <div className="space-y-2.5">
              <Skeleton className="h-2.5 w-full rounded-full" />
              <div className="flex gap-2">
                {Array.from({ length: 4 }).map((_, i) => (
                  <Skeleton key={i} className="h-5 w-16 rounded-md" />
                ))}
              </div>
            </div>
          ) : (
            <ProtocolBar
              byProtocol={nodeStats?.by_protocol ?? {}}
              total={nodeStats?.total ?? 0}
            />
          )}
        </CardContent>
      </Card>

      {/* Subscription health */}
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Wifi className="size-4 text-muted-foreground" />
          <h2 className="font-heading text-sm font-semibold">Subscription Health</h2>
        </div>
        {subsLoading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <SubscriptionCardSkeleton key={i} />
            ))}
          </div>
        ) : subscriptions && subscriptions.length > 0 ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {subscriptions.map((sub) => (
              <SubscriptionHealthCard key={sub.id} sub={sub} />
            ))}
          </div>
        ) : (
          <Card>
            <CardContent className="py-8 text-center text-sm text-muted-foreground">
              No subscriptions yet. Add one to get started.
            </CardContent>
          </Card>
        )}
      </div>

      {/* Quick links */}
      <div>
        <h2 className="mb-3 font-heading text-sm font-semibold text-muted-foreground">
          Quick Links
        </h2>
        <div className="flex flex-wrap gap-3">
          <Button variant="outline" size="sm" render={<Link to="/converter" />}>
            Subscription Converter
            <ArrowRight className="size-3.5" />
          </Button>
          <Button variant="outline" size="sm" render={<Link to="/subscriptions" />}>
            Manage Subscriptions
            <ArrowRight className="size-3.5" />
          </Button>
          <Button variant="outline" size="sm" render={<Link to="/nodes" />}>
            View Nodes
            <ArrowRight className="size-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
