import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, put, del, patch } from "@/lib/api";
import type { Node, NodeStats } from "@/types";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Plus, Trash2, Pencil, Upload, Copy, Loader2, Server, CheckSquare, XSquare } from "lucide-react";

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

export default function NodesPage() {
  const qc = useQueryClient();
  const [filter, setFilter] = useState({ protocol: "", enabled: "", search: "", source: "" });
  const [dialogOpen, setDialogOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [importText, setImportText] = useState("");
  const [form, setForm] = useState({ name: "", protocol: "trojan", server: "", port: 443, config: "{}", enabled: true, tags: "" });

  const { data: nodes, isLoading } = useQuery({
    queryKey: ["nodes"],
    queryFn: () => get<Node[]>("/api/nodes"),
  });
  const { data: stats } = useQuery({
    queryKey: ["nodeStats"],
    queryFn: () => get<NodeStats>("/api/nodes/stats"),
  });

  const filtered = useMemo(() => {
    if (!nodes) return [];
    return nodes.filter((n) => {
      if (filter.protocol && n.protocol !== filter.protocol) return false;
      if (filter.enabled === "true" && !n.enabled) return false;
      if (filter.enabled === "false" && n.enabled) return false;
      if (filter.search && !n.name.toLowerCase().includes(filter.search.toLowerCase()) && !n.server.toLowerCase().includes(filter.search.toLowerCase())) return false;
      if (filter.source === "manual" && n.source_id !== null) return false;
      if (filter.source && filter.source !== "manual" && n.source_name !== filter.source) return false;
      return true;
    });
  }, [nodes, filter]);

  const protocols = useMemo(() => {
    if (!nodes) return [];
    return [...new Set(nodes.map((n) => n.protocol))].sort();
  }, [nodes]);

  const sources = useMemo(() => {
    if (!nodes) return [];
    return [...new Set(nodes.filter((n) => n.source_id !== null).map((n) => n.source_name))].sort();
  }, [nodes]);

  const saveMut = useMutation({
    mutationFn: (data: Record<string, unknown>) =>
      editId ? put(`/api/nodes/${editId}`, data) : post("/api/nodes", data),
    onSuccess: () => {
      toast.success(editId ? "Updated" : "Created");
      qc.invalidateQueries({ queryKey: ["nodes"] });
      qc.invalidateQueries({ queryKey: ["nodeStats"] });
      setDialogOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => del(`/api/nodes/${id}`),
    onSuccess: () => {
      toast.success("Deleted");
      qc.invalidateQueries({ queryKey: ["nodes"] });
      qc.invalidateQueries({ queryKey: ["nodeStats"] });
      setDeleteId(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const importMut = useMutation({
    mutationFn: (links: string) => post("/api/nodes/import", { content: links }),
    onSuccess: () => {
      toast.success("Nodes imported");
      qc.invalidateQueries({ queryKey: ["nodes"] });
      qc.invalidateQueries({ queryKey: ["nodeStats"] });
      setImportOpen(false);
      setImportText("");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const batchMut = useMutation({
    mutationFn: (data: { action: string; ids: number[] }) => patch("/api/nodes/batch", data),
    onSuccess: () => {
      toast.success("Batch operation complete");
      qc.invalidateQueries({ queryKey: ["nodes"] });
      qc.invalidateQueries({ queryKey: ["nodeStats"] });
      setSelected(new Set());
    },
    onError: (e: Error) => toast.error(e.message),
  });

  function openCreate() {
    setEditId(null);
    setForm({ name: "", protocol: "trojan", server: "", port: 443, config: "{}", enabled: true, tags: "" });
    setDialogOpen(true);
  }

  function openEdit(node: Node) {
    setEditId(node.id);
    setForm({
      name: node.name, protocol: node.protocol, server: node.server, port: node.port,
      config: JSON.stringify(node.config || {}, null, 2), enabled: node.enabled,
      tags: (node.tags || []).join(", "),
    });
    setDialogOpen(true);
  }

  function handleSave() {
    let cfg = {};
    try { cfg = JSON.parse(form.config); } catch { toast.error("Invalid JSON config"); return; }
    saveMut.mutate({
      name: form.name, protocol: form.protocol, server: form.server, port: form.port,
      config: cfg, enabled: form.enabled,
      tags: form.tags.split(",").map((s) => s.trim()).filter(Boolean),
    });
  }

  function toggleSelect(id: number) {
    setSelected((s) => { const n = new Set(s); n.has(id) ? n.delete(id) : n.add(id); return n; });
  }

  function toggleAll() {
    if (selected.size === filtered.length) setSelected(new Set());
    else setSelected(new Set(filtered.map((n) => n.id)));
  }

  async function copyShareUrl(id: number) {
    try {
      const data = await get<{ share_url: string }>(`/api/nodes/${id}/share`);
      await navigator.clipboard.writeText(data.share_url);
      toast.success("Share URL copied");
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Failed");
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">Nodes</h1>
          <p className="text-sm text-muted-foreground">Manage proxy nodes</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => setImportOpen(true)}><Upload className="size-4 mr-1.5" /> Import</Button>
          <Button size="sm" onClick={openCreate}><Plus className="size-4 mr-1.5" /> New</Button>
        </div>
      </div>

      {/* Stats bar */}
      {stats && (
        <div className="flex flex-wrap gap-3">
          <Badge variant="outline"><Server className="size-3 mr-1" /> {stats.total} total</Badge>
          <Badge variant="default">{stats.enabled} enabled</Badge>
          <Badge variant="secondary">{stats.disabled} disabled</Badge>
          {Object.entries(stats.by_protocol || {}).slice(0, 6).map(([p, c]) => (
            <Badge key={p} variant="outline">{p}: {c}</Badge>
          ))}
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <Input placeholder="Search..." className="w-48" value={filter.search} onChange={(e) => setFilter((f) => ({ ...f, search: e.target.value }))} />
        <Select value={filter.protocol} onValueChange={(v) => setFilter((f) => ({ ...f, protocol: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-36"><SelectValue placeholder="Protocol" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All protocols</SelectItem>
            {protocols.map((p) => <SelectItem key={p} value={p}>{p}</SelectItem>)}
          </SelectContent>
        </Select>
        <Select value={filter.enabled || "all"} onValueChange={(v) => setFilter((f) => ({ ...f, enabled: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-32"><SelectValue placeholder="Status" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="true">Enabled</SelectItem>
            <SelectItem value="false">Disabled</SelectItem>
          </SelectContent>
        </Select>
        <Select value={filter.source || "all"} onValueChange={(v) => setFilter((f) => ({ ...f, source: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-40"><SelectValue placeholder="Source" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All sources</SelectItem>
            <SelectItem value="manual">Manual</SelectItem>
            {sources.map((s) => <SelectItem key={s} value={s}>{s}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>

      {/* Batch actions */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">{selected.size} selected</span>
          <Button size="sm" variant="outline" onClick={() => batchMut.mutate({ action: "enable", ids: [...selected] })}>
            <CheckSquare className="size-4 mr-1" /> Enable
          </Button>
          <Button size="sm" variant="outline" onClick={() => batchMut.mutate({ action: "disable", ids: [...selected] })}>
            <XSquare className="size-4 mr-1" /> Disable
          </Button>
        </div>
      )}

      {isLoading ? (
        <Skeleton className="h-64 rounded-xl" />
      ) : (
        <Card className="overflow-hidden">
          <Table containerClassName="max-h-[calc(100vh-280px)] overflow-y-auto">
            <TableHeader className="sticky top-0 z-10 bg-card">
              <TableRow>
                <TableHead className="w-10"><Checkbox checked={selected.size === filtered.length && filtered.length > 0} onCheckedChange={toggleAll} /></TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Protocol</TableHead>
                <TableHead>Server</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Synced</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((node) => (
                <TableRow key={node.id}>
                  <TableCell><Checkbox checked={selected.has(node.id)} onCheckedChange={() => toggleSelect(node.id)} /></TableCell>
                  <TableCell className="font-medium max-w-[200px] truncate">{node.name}</TableCell>
                  <TableCell><Badge variant="outline">{node.protocol}</Badge></TableCell>
                  <TableCell className="text-muted-foreground text-sm">{node.server}:{node.port}</TableCell>
                  <TableCell><Badge variant={node.enabled ? "default" : "secondary"}>{node.enabled ? "On" : "Off"}</Badge></TableCell>
                  <TableCell className="text-sm text-muted-foreground">{timeAgo(node.last_synced_at)}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="sm" className="h-7 px-2" onClick={() => copyShareUrl(node.id)}><Copy className="size-3" /></Button>
                      <Button variant="ghost" size="sm" className="h-7 px-2" onClick={() => openEdit(node)}><Pencil className="size-3" /></Button>
                      <Button variant="ghost" size="sm" className="h-7 px-2 text-destructive" onClick={() => setDeleteId(node.id)}><Trash2 className="size-3" /></Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {!filtered.length && (
                <TableRow><TableCell colSpan={7} className="text-center text-muted-foreground py-8">No nodes found</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </Card>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{editId ? "Edit Node" : "New Node"}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2"><Label>Name</Label><Input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} /></div>
            <div className="space-y-2">
              <Label>Protocol</Label>
              <Select value={form.protocol} onValueChange={(v) => setForm((f) => ({ ...f, protocol: v }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {["trojan", "vmess", "vless", "ss", "ssr", "wireguard", "hysteria", "hysteria2", "tuic"].map((p) => (
                    <SelectItem key={p} value={p}>{p}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2 space-y-2"><Label>Server</Label><Input value={form.server} onChange={(e) => setForm((f) => ({ ...f, server: e.target.value }))} /></div>
              <div className="space-y-2"><Label>Port</Label><Input type="number" value={form.port} onChange={(e) => setForm((f) => ({ ...f, port: Number(e.target.value) }))} /></div>
            </div>
            <div className="space-y-2"><Label>Config (JSON)</Label><Textarea value={form.config} onChange={(e) => setForm((f) => ({ ...f, config: e.target.value }))} rows={6} className="font-mono text-xs" /></div>
            <div className="space-y-2"><Label>Tags (comma-separated)</Label><Input value={form.tags} onChange={(e) => setForm((f) => ({ ...f, tags: e.target.value }))} /></div>
            <div className="flex items-center justify-between"><Label>Enabled</Label><Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleSave} disabled={saveMut.isPending}>
              {saveMut.isPending && <Loader2 className="size-4 mr-1.5 animate-spin" />}
              {editId ? "Save" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Import Dialog */}
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Import Nodes</DialogTitle></DialogHeader>
          <div className="space-y-2">
            <Label>Paste share links (one per line)</Label>
            <Textarea value={importText} onChange={(e) => setImportText(e.target.value)} rows={8} placeholder="ss://...\nvmess://...\ntrojan://..." className="font-mono text-xs" />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>Cancel</Button>
            <Button onClick={() => importMut.mutate(importText)} disabled={importMut.isPending || !importText.trim()}>
              {importMut.isPending && <Loader2 className="size-4 mr-1.5 animate-spin" />} Import
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <Dialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>Delete Node</DialogTitle></DialogHeader>
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
