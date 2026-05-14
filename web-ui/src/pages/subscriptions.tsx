import { useState, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, put, del } from "@/lib/api";
import type { Subscription, Node } from "@/types";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
  AlertCircle,
  Loader2,
  Server,
  Clock,
  ChevronDown,
  Filter,
  Rss,
  CalendarClock,
} from "lucide-react";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const INTERVALS = [
  { label: "5 min", value: "300" },
  { label: "10 min", value: "600" },
  { label: "30 min", value: "1800" },
  { label: "1 hour", value: "3600" },
  { label: "2 hours", value: "7200" },
  { label: "6 hours", value: "21600" },
  { label: "12 hours", value: "43200" },
  { label: "24 hours", value: "86400" },
];

const PROTOCOLS = [
  "trojan",
  "vmess",
  "vless",
  "ss",
  "ssr",
  "wireguard",
  "hysteria",
  "hysteria2",
  "tuic",
];

const PROTOCOL_COLORS: Record<string, string> = {
  trojan: "bg-blue-500/20 text-blue-400",
  vmess: "bg-violet-500/20 text-violet-400",
  vless: "bg-indigo-500/20 text-indigo-400",
  ss: "bg-emerald-500/20 text-emerald-400",
  ssr: "bg-teal-500/20 text-teal-400",
  wireguard: "bg-orange-500/20 text-orange-400",
  hysteria: "bg-pink-500/20 text-pink-400",
  hysteria2: "bg-rose-500/20 text-rose-400",
  tuic: "bg-amber-500/20 text-amber-400",
};

// ---------------------------------------------------------------------------
// Form type
// ---------------------------------------------------------------------------

interface SubscriptionForm {
  name: string;
  url: string;
  auto_refresh: boolean;
  refresh_interval: number;
  description: string;
  disable_name_prefix: boolean;
  filter_rules: {
    exclude_keywords: string[];
    exclude_regex: string;
    include_protocols: string[];
  };
}

const EMPTY_FORM: SubscriptionForm = {
  name: "",
  url: "",
  auto_refresh: false,
  refresh_interval: 3600,
  description: "",
  disable_name_prefix: false,
  filter_rules: {
    exclude_keywords: [],
    exclude_regex: "",
    include_protocols: [],
  },
};

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

function protocolPillClass(proto: string): string {
  return PROTOCOL_COLORS[proto.toLowerCase()] ?? "bg-muted text-muted-foreground";
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function CardSkeleton() {
  return (
    <Card>
      <CardContent className="space-y-4 pt-1">
        <div className="flex items-start justify-between">
          <div className="space-y-1.5">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="h-3.5 w-56" />
          </div>
          <Skeleton className="h-5 w-12 rounded-full" />
        </div>
        <div className="flex gap-4">
          <Skeleton className="h-3.5 w-20" />
          <Skeleton className="h-3.5 w-24" />
        </div>
        <Skeleton className="h-7 w-full" />
      </CardContent>
    </Card>
  );
}

function ProtocolPills({ nodes }: { nodes: Node[] }) {
  if (nodes.length === 0) return null;

  const counts: Record<string, number> = {};
  for (const n of nodes) {
    counts[n.protocol] = (counts[n.protocol] ?? 0) + 1;
  }

  const sorted = Object.entries(counts).sort(([, a], [, b]) => b - a);

  return (
    <div className="flex flex-wrap gap-1">
      {sorted.map(([proto, count]) => (
        <span
          key={proto}
          className={cn(
            "inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[0.65rem] font-medium",
            protocolPillClass(proto),
          )}
        >
          {proto}
          <span className="opacity-70">{count}</span>
        </span>
      ))}
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
          className={cn(
            "h-full rounded-full transition-all",
            high ? "bg-orange-500" : "bg-emerald-500",
          )}
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

function SubscriptionCard({
  sub,
  nodes,
  onEdit,
  onSync,
  onDelete,
  syncing,
}: {
  sub: Subscription;
  nodes: Node[];
  onEdit: () => void;
  onSync: () => void;
  onDelete: () => void;
  syncing: boolean;
}) {
  return (
    <Card>
      <CardContent className="space-y-3 pt-1">
        {/* Header */}
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <h3 className="truncate font-heading text-sm font-semibold">
              {sub.name}
            </h3>
            <p className="truncate text-xs text-muted-foreground" title={sub.url}>
              {sub.url}
            </p>
          </div>
          <Badge
            variant={sub.enabled ? "default" : "secondary"}
            className="shrink-0 text-[0.65rem]"
          >
            {sub.enabled ? "Enabled" : "Disabled"}
          </Badge>
        </div>

        {/* Error */}
        {sub.last_fetch_error && (
          <div className="flex items-start gap-1.5 rounded-md bg-red-500/10 px-2.5 py-1.5 text-xs text-red-400">
            <AlertCircle className="mt-0.5 size-3 shrink-0" />
            <span className="line-clamp-2">{sub.last_fetch_error}</span>
          </div>
        )}

        {/* Meta row */}
        <div className="flex items-center gap-4 text-xs text-muted-foreground">
          <span className="flex items-center gap-1">
            <Server className="size-3" />
            {sub.node_count} nodes
          </span>
          <span className="flex items-center gap-1">
            <Clock className="size-3" />
            {relativeTime(sub.last_fetched_at)}
          </span>
        </div>

        {/* Traffic */}
        <TrafficBar sub={sub} />

        {/* Expiry */}
        <ExpiryInfo sub={sub} />

        {/* Protocol pills */}
        <ProtocolPills nodes={nodes} />

        {/* Actions */}
        <Separator />
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="xs"
            onClick={onEdit}
          >
            <Pencil className="size-3" />
            Edit
          </Button>
          <Button
            variant="ghost"
            size="xs"
            onClick={onSync}
            disabled={syncing}
          >
            {syncing ? (
              <Loader2 className="size-3 animate-spin" />
            ) : (
              <RefreshCw className="size-3" />
            )}
            Sync
          </Button>
          <Button
            variant="destructive"
            size="xs"
            onClick={onDelete}
          >
            <Trash2 className="size-3" />
            Delete
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function SubscriptionsPage() {
  const queryClient = useQueryClient();

  // ---- State ----
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<SubscriptionForm>(EMPTY_FORM);
  const [excludeKeywordsText, setExcludeKeywordsText] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Subscription | null>(null);
  const [syncingId, setSyncingId] = useState<number | null>(null);

  // ---- Queries ----
  const { data: subscriptions, isLoading } = useQuery({
    queryKey: ["subscriptions"],
    queryFn: () => get<Subscription[]>("/api/subscriptions"),
  });

  const { data: allNodes } = useQuery({
    queryKey: ["nodes"],
    queryFn: () => get<Node[]>("/api/nodes"),
  });

  // Build a map: subscription source_id → nodes
  const nodesBySource = allNodes
    ? allNodes.reduce<Record<number, Node[]>>((acc, node) => {
        if (node.source_id != null) {
          (acc[node.source_id] ??= []).push(node);
        }
        return acc;
      }, {})
    : {};

  // ---- Mutations ----
  const saveMutation = useMutation({
    mutationFn: (data: SubscriptionForm) =>
      editId
        ? put(`/api/subscriptions/${editId}`, data)
        : post("/api/subscriptions", data),
    onSuccess: () => {
      toast.success(editId ? "Subscription updated" : "Subscription created");
      queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
      setDialogOpen(false);
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => del(`/api/subscriptions/${id}`),
    onSuccess: () => {
      toast.success("Subscription deleted");
      queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
      setDeleteTarget(null);
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const syncAllMutation = useMutation({
    mutationFn: () => post("/api/subscriptions/sync"),
    onSuccess: () => {
      toast.success("Sync all triggered");
      queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  // ---- Handlers ----
  const openCreate = useCallback(() => {
    setEditId(null);
    setForm(EMPTY_FORM);
    setExcludeKeywordsText("");
    setFiltersOpen(false);
    setDialogOpen(true);
  }, []);

  const openEdit = useCallback((sub: Subscription) => {
    setEditId(sub.id);
    const kw = sub.filter_rules?.exclude_keywords ?? [];
    setForm({
      name: sub.name,
      url: sub.url,
      auto_refresh: sub.auto_refresh,
      refresh_interval: sub.refresh_interval,
      description: sub.description,
      disable_name_prefix: sub.disable_name_prefix,
      filter_rules: {
        exclude_keywords: kw,
        exclude_regex: sub.filter_rules?.exclude_regex ?? "",
        include_protocols: sub.filter_rules?.include_protocols ?? [],
      },
    });
    setExcludeKeywordsText(kw.join(", "));
    setFiltersOpen(kw.length > 0 || !!sub.filter_rules?.exclude_regex || (sub.filter_rules?.include_protocols?.length ?? 0) > 0);
    setDialogOpen(true);
  }, []);

  const handleSync = useCallback(
    async (id: number) => {
      setSyncingId(id);
      try {
        await post(`/api/subscriptions/${id}/sync`);
        toast.success("Sync triggered");
        queryClient.invalidateQueries({ queryKey: ["subscriptions"] });
      } catch (err: unknown) {
        toast.error(err instanceof Error ? err.message : "Sync failed");
      } finally {
        setSyncingId(null);
      }
    },
    [queryClient],
  );

  const handleSave = useCallback(() => {
    const payload: SubscriptionForm = {
      ...form,
      filter_rules: {
        ...form.filter_rules,
        exclude_keywords: excludeKeywordsText
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      },
    };
    saveMutation.mutate(payload);
  }, [form, excludeKeywordsText, saveMutation]);

  const toggleProtocol = useCallback((proto: string) => {
    setForm((prev) => {
      const current = prev.filter_rules.include_protocols;
      const next = current.includes(proto)
        ? current.filter((p) => p !== proto)
        : [...current, proto];
      return {
        ...prev,
        filter_rules: { ...prev.filter_rules, include_protocols: next },
      };
    });
  }, []);

  const updateField = useCallback(
    <K extends keyof SubscriptionForm>(key: K, value: SubscriptionForm[K]) => {
      setForm((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  // ---- Render ----
  return (
    <div className="space-y-6 p-4 lg:p-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight text-foreground">
            Subscriptions
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage your proxy subscription sources
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => syncAllMutation.mutate()}
            disabled={syncAllMutation.isPending}
          >
            <RefreshCw
              className={cn("size-3.5", syncAllMutation.isPending && "animate-spin")}
            />
            Sync All
          </Button>
          <Button size="sm" onClick={openCreate}>
            <Plus className="size-3.5" />
            New Subscription
          </Button>
        </div>
      </div>

      {/* List */}
      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      ) : !subscriptions?.length ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-16">
            <div className="flex size-12 items-center justify-center rounded-full bg-muted">
              <Rss className="size-6 text-muted-foreground" />
            </div>
            <div className="text-center">
              <p className="text-sm font-medium text-foreground">
                No subscriptions
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                Add your first subscription to get started.
              </p>
            </div>
            <Button size="sm" onClick={openCreate} className="mt-1">
              <Plus className="size-3.5" />
              New Subscription
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {subscriptions.map((sub) => (
            <SubscriptionCard
              key={sub.id}
              sub={sub}
              nodes={nodesBySource[sub.id] ?? []}
              onEdit={() => openEdit(sub)}
              onSync={() => handleSync(sub.id)}
              onDelete={() => setDeleteTarget(sub)}
              syncing={syncingId === sub.id}
            />
          ))}
        </div>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* Create / Edit Dialog                                                */}
      {/* ------------------------------------------------------------------ */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editId ? "Edit Subscription" : "New Subscription"}
            </DialogTitle>
            <DialogDescription>
              {editId
                ? "Update the subscription settings below."
                : "Add a new proxy subscription source."}
            </DialogDescription>
          </DialogHeader>

          <div className="max-h-[60vh] space-y-4 overflow-y-auto pr-1">
            {/* Name */}
            <div className="space-y-1.5">
              <Label htmlFor="sub-name">
                Name <span className="text-destructive">*</span>
              </Label>
              <Input
                id="sub-name"
                placeholder="My Subscription"
                value={form.name}
                onChange={(e) => updateField("name", e.target.value)}
              />
            </div>

            {/* URL */}
            <div className="space-y-1.5">
              <Label htmlFor="sub-url">
                URL <span className="text-destructive">*</span>
              </Label>
              <Input
                id="sub-url"
                placeholder="https://example.com/sub?token=..."
                value={form.url}
                onChange={(e) => updateField("url", e.target.value)}
              />
            </div>

            {/* Auto Refresh + Interval */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label htmlFor="sub-auto-refresh">Auto Refresh</Label>
                <Switch
                  id="sub-auto-refresh"
                  checked={form.auto_refresh}
                  onCheckedChange={(v) => updateField("auto_refresh", v)}
                />
              </div>
              {form.auto_refresh && (
                <div className="space-y-1.5">
                  <Label>Refresh Interval</Label>
                  <Select
                    value={String(form.refresh_interval)}
                    onValueChange={(v) =>
                      updateField("refresh_interval", Number(v))
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {INTERVALS.map((i) => (
                        <SelectItem key={i.value} value={i.value}>
                          {i.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>

            <Separator />

            {/* Filter Rules (collapsible) */}
            <div>
              <button
                type="button"
                className="flex w-full items-center gap-2 text-sm font-medium text-foreground"
                onClick={() => setFiltersOpen((v) => !v)}
              >
                <Filter className="size-3.5 text-muted-foreground" />
                Filter Rules
                <ChevronDown
                  className={cn(
                    "ml-auto size-4 text-muted-foreground transition-transform",
                    filtersOpen && "rotate-180",
                  )}
                />
              </button>

              {filtersOpen && (
                <div className="mt-3 space-y-4 rounded-lg border bg-muted/30 p-3">
                  {/* Exclude Keywords */}
                  <div className="space-y-1.5">
                    <Label htmlFor="sub-excl-kw">
                      Exclude Keywords
                    </Label>
                    <Textarea
                      id="sub-excl-kw"
                      placeholder="keyword1, keyword2, ..."
                      value={excludeKeywordsText}
                      onChange={(e) => setExcludeKeywordsText(e.target.value)}
                      rows={2}
                    />
                    <p className="text-[0.65rem] text-muted-foreground">
                      Comma-separated. Nodes matching these keywords will be
                      excluded.
                    </p>
                  </div>

                  {/* Exclude Regex */}
                  <div className="space-y-1.5">
                    <Label htmlFor="sub-excl-regex">Exclude Regex</Label>
                    <Input
                      id="sub-excl-regex"
                      placeholder="e.g. .*expired.*"
                      value={form.filter_rules.exclude_regex}
                      onChange={(e) =>
                        setForm((prev) => ({
                          ...prev,
                          filter_rules: {
                            ...prev.filter_rules,
                            exclude_regex: e.target.value,
                          },
                        }))
                      }
                    />
                  </div>

                  {/* Include Protocols */}
                  <div className="space-y-2">
                    <Label>Include Protocols</Label>
                    <div className="grid grid-cols-3 gap-2">
                      {PROTOCOLS.map((proto) => {
                        const checked =
                          form.filter_rules.include_protocols.includes(proto);
                        return (
                          <label
                            key={proto}
                            className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs transition-colors hover:bg-muted"
                          >
                            <Checkbox
                              checked={checked}
                              onCheckedChange={() => toggleProtocol(proto)}
                            />
                            <span className="select-none">{proto}</span>
                          </label>
                        );
                      })}
                    </div>
                    <p className="text-[0.65rem] text-muted-foreground">
                      Leave all unchecked to include every protocol.
                    </p>
                  </div>
                </div>
              )}
            </div>

            <Separator />

            {/* Description */}
            <div className="space-y-1.5">
              <Label htmlFor="sub-desc">Description</Label>
              <Textarea
                id="sub-desc"
                placeholder="Optional notes..."
                value={form.description}
                onChange={(e) => updateField("description", e.target.value)}
                rows={2}
              />
            </div>

            {/* Disable Name Prefix */}
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label htmlFor="sub-prefix">Disable Name Prefix</Label>
                <p className="text-[0.65rem] text-muted-foreground">
                  Don't prepend subscription name to node names
                </p>
              </div>
              <Switch
                id="sub-prefix"
                checked={form.disable_name_prefix}
                onCheckedChange={(v) => updateField("disable_name_prefix", v)}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleSave}
              disabled={saveMutation.isPending || !form.name || !form.url}
            >
              {saveMutation.isPending && (
                <Loader2 className="size-3.5 animate-spin" />
              )}
              {editId ? "Save Changes" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ------------------------------------------------------------------ */}
      {/* Delete Confirmation Dialog                                          */}
      {/* ------------------------------------------------------------------ */}
      <Dialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete Subscription</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete{" "}
              <span className="font-semibold text-foreground">
                {deleteTarget?.name}
              </span>
              ? This action cannot be undone. All associated nodes will also be
              removed.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending && (
                <Loader2 className="size-3.5 animate-spin" />
              )}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
