import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, put, del } from "@/lib/api";
import type { RuleSource } from "@/types";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Plus, Pencil, Trash2, RefreshCw, Copy, Loader2, BookOpen, AlertCircle } from "lucide-react";

const FORMATS = ["sing-box", "surge", "clash-classical", "clash-domain", "clash-ipcidr", "other"];
const EXPORT_TARGETS = [
  { key: "sing-box", label: "sing-box" },
  { key: "surge", label: "Surge" },
  { key: "clash-classical", label: "Clash" },
] as const;
const INTERVALS = [
  { label: "30 min", value: "1800" },
  { label: "1 hour", value: "3600" },
  { label: "6 hours", value: "21600" },
  { label: "12 hours", value: "43200" },
  { label: "24 hours", value: "86400" },
];

const emptyForm = { name: "", description: "", url: "", raw_content: "", source_format: "clash-classical", enabled: true, auto_refresh: false, refresh_interval: 3600 };

function timeAgo(d: string | null) {
  if (!d) return "Never";
  const ms = Date.now() - new Date(d).getTime();
  const m = Math.floor(ms / 60000);
  if (m < 1) return "Just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

export default function RuleSourcesPage() {
  const qc = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [syncingId, setSyncingId] = useState<number | null>(null);

  const { data: sources, isLoading } = useQuery({
    queryKey: ["ruleSources"],
    queryFn: () => get<RuleSource[]>("/api/rule-sources"),
  });

  const saveMut = useMutation({
    mutationFn: (data: typeof form) =>
      editId ? put(`/api/rule-sources/${editId}`, data) : post("/api/rule-sources", data),
    onSuccess: () => {
      toast.success(editId ? "Updated" : "Created");
      qc.invalidateQueries({ queryKey: ["ruleSources"] });
      setDialogOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => del(`/api/rule-sources/${id}`),
    onSuccess: () => {
      toast.success("Deleted");
      qc.invalidateQueries({ queryKey: ["ruleSources"] });
      setDeleteId(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  function openCreate() { setEditId(null); setForm(emptyForm); setDialogOpen(true); }
  function openEdit(s: RuleSource) {
    setEditId(s.id);
    setForm({ name: s.name, description: s.description, url: s.url, raw_content: s.raw_content || "", source_format: s.source_format, enabled: s.enabled, auto_refresh: s.auto_refresh, refresh_interval: s.refresh_interval });
    setDialogOpen(true);
  }

  async function handleSync(id: number) {
    setSyncingId(id);
    try {
      await post(`/api/rule-sources/${id}/sync`);
      toast.success("Sync triggered");
      qc.invalidateQueries({ queryKey: ["ruleSources"] });
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : "Sync failed"); }
    finally { setSyncingId(null); }
  }

  async function copyExportUrl(name: string, target: string, label: string) {
    const url = `${window.location.origin}/rulesets/${encodeURIComponent(name)}?target=${encodeURIComponent(target)}`;
    await navigator.clipboard.writeText(url);
    toast.success(`${label} export URL copied`);
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">Rule Sources</h1>
          <p className="text-sm text-muted-foreground">Manage external rule sources</p>
        </div>
        <Button size="sm" onClick={openCreate}><Plus className="size-4 mr-1.5" /> New</Button>
      </div>

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2">{[1, 2].map((i) => <Skeleton key={i} className="h-36 rounded-xl" />)}</div>
      ) : !sources?.length ? (
        <Card><CardContent className="py-12 text-center text-muted-foreground">No rule sources yet.</CardContent></Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {sources.map((s) => (
            <Card key={s.id}>
              <CardContent className="pt-4 space-y-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <BookOpen className="size-4 text-muted-foreground shrink-0" />
                      <span className="font-medium truncate">{s.name}</span>
                    </div>
                    <div className="text-xs text-muted-foreground truncate mt-1">{s.url}</div>
                  </div>
                  <div className="flex gap-1.5 shrink-0">
                    <Badge variant="outline">{s.source_format}</Badge>
                    <Badge variant={s.enabled ? "default" : "secondary"}>{s.enabled ? "On" : "Off"}</Badge>
                  </div>
                </div>
                {s.last_sync_error && (
                  <div className="flex items-start gap-1.5 text-xs text-destructive">
                    <AlertCircle className="size-3.5 mt-0.5 shrink-0" />
                    <span className="truncate">{s.last_sync_error}</span>
                  </div>
                )}
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{s.rule_count} rules</span>
                  <span>Synced: {timeAgo(s.last_synced_at)}</span>
                </div>
                <div className="flex flex-wrap gap-1.5 pt-1">
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => openEdit(s)}><Pencil className="size-3 mr-1" /> Edit</Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => handleSync(s.id)} disabled={syncingId === s.id}>
                    {syncingId === s.id ? <Loader2 className="size-3 mr-1 animate-spin" /> : <RefreshCw className="size-3 mr-1" />} Sync
                  </Button>
                  {EXPORT_TARGETS.map((t) => (
                    <Button key={t.key} variant="outline" size="sm" className="h-7 px-2 text-xs" onClick={() => copyExportUrl(s.name, t.key, t.label)}>
                      <Copy className="size-3 mr-1" /> {t.label}
                    </Button>
                  ))}
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-destructive hover:text-destructive" onClick={() => setDeleteId(s.id)}><Trash2 className="size-3 mr-1" /> Delete</Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{editId ? "Edit Rule Source" : "New Rule Source"}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2"><Label>Name</Label><Input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} /></div>
            <div className="space-y-2"><Label>URL</Label><Input value={form.url} onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))} placeholder="Remote rule source URL (leave empty for inline rules)" /></div>
            <div className="space-y-2">
              <Label>Rule Content</Label>
              <Textarea
                value={form.raw_content}
                onChange={(e) => setForm((f) => ({ ...f, raw_content: e.target.value }))}
                rows={10}
                className="min-h-[200px] font-mono text-xs"
                placeholder="每行一条规则（支持 # 或 // 开头的注释行）"
              />
            </div>
            <div className="space-y-2">
              <Label>Format</Label>
              <Select value={form.source_format} onValueChange={(v) => setForm((f) => ({ ...f, source_format: v }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{FORMATS.map((f) => <SelectItem key={f} value={f}>{f}</SelectItem>)}</SelectContent>
              </Select>
            </div>
            <div className="space-y-2"><Label>Description</Label><Textarea value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} rows={2} /></div>
            <div className="flex items-center justify-between"><Label>Enabled</Label><Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} /></div>
            <div className="flex items-center justify-between"><Label>Auto Refresh</Label><Switch checked={form.auto_refresh} onCheckedChange={(v) => setForm((f) => ({ ...f, auto_refresh: v }))} /></div>
            {form.auto_refresh && (
              <div className="space-y-2">
                <Label>Interval</Label>
                <Select value={String(form.refresh_interval)} onValueChange={(v) => setForm((f) => ({ ...f, refresh_interval: Number(v) }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>{INTERVALS.map((i) => <SelectItem key={i.value} value={i.value}>{i.label}</SelectItem>)}</SelectContent>
                </Select>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button onClick={() => saveMut.mutate(form)} disabled={saveMut.isPending}>
              {saveMut.isPending && <Loader2 className="size-4 mr-1.5 animate-spin" />}{editId ? "Save" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>Delete Rule Source</DialogTitle></DialogHeader>
          <p className="text-sm text-muted-foreground">This cannot be undone.</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>Cancel</Button>
            <Button variant="destructive" onClick={() => deleteId && deleteMut.mutate(deleteId)} disabled={deleteMut.isPending}>Delete</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
