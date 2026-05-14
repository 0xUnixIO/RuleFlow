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
  if (!d) return "从未";
  const ms = Date.now() - new Date(d).getTime();
  const m = Math.floor(ms / 60000);
  if (m < 1) return "刚刚";
  if (m < 60) return `${m} 分钟前`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} 小时前`;
  return `${Math.floor(h / 24)} 天前`;
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
      toast.success(editId ? "已更新" : "已创建");
      qc.invalidateQueries({ queryKey: ["nodes"] });
      qc.invalidateQueries({ queryKey: ["nodeStats"] });
      setDialogOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => del(`/api/nodes/${id}`),
    onSuccess: () => {
      toast.success("已删除");
      qc.invalidateQueries({ queryKey: ["nodes"] });
      qc.invalidateQueries({ queryKey: ["nodeStats"] });
      setDeleteId(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const importMut = useMutation({
    mutationFn: (links: string) => post("/api/nodes/import", { content: links }),
    onSuccess: () => {
      toast.success("节点导入成功");
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
      toast.success("批量操作完成");
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
    try { cfg = JSON.parse(form.config); } catch { toast.error("JSON 配置格式无效"); return; }
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
      toast.success("分享链接已复制");
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "操作失败");
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">节点</h1>
          <p className="text-sm text-muted-foreground">管理代理节点</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => setImportOpen(true)}><Upload className="size-4 mr-1.5" /> 导入</Button>
          <Button size="sm" onClick={openCreate}><Plus className="size-4 mr-1.5" /> 新建</Button>
        </div>
      </div>

      {/* Stats bar */}
      {stats && (
        <div className="flex flex-wrap gap-3">
          <Badge variant="outline"><Server className="size-3 mr-1" /> {stats.total} 个</Badge>
          <Badge variant="default">{stats.enabled} 已启用</Badge>
          <Badge variant="secondary">{stats.disabled} 已禁用</Badge>
          {Object.entries(stats.by_protocol || {}).slice(0, 6).map(([p, c]) => (
            <Badge key={p} variant="outline">{p}: {c}</Badge>
          ))}
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <Input placeholder="搜索…" className="w-48" value={filter.search} onChange={(e) => setFilter((f) => ({ ...f, search: e.target.value }))} />
        <Select value={filter.protocol} onValueChange={(v) => setFilter((f) => ({ ...f, protocol: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-36"><SelectValue placeholder="协议" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部协议</SelectItem>
            {protocols.map((p) => <SelectItem key={p} value={p}>{p}</SelectItem>)}
          </SelectContent>
        </Select>
        <Select value={filter.enabled || "all"} onValueChange={(v) => setFilter((f) => ({ ...f, enabled: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-32"><SelectValue placeholder="状态" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部</SelectItem>
            <SelectItem value="true">已启用</SelectItem>
            <SelectItem value="false">已禁用</SelectItem>
          </SelectContent>
        </Select>
        <Select value={filter.source || "all"} onValueChange={(v) => setFilter((f) => ({ ...f, source: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-40"><SelectValue placeholder="来源" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部来源</SelectItem>
            <SelectItem value="manual">手动添加</SelectItem>
            {sources.map((s) => <SelectItem key={s} value={s}>{s}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>

      {/* Batch actions */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">已选 {selected.size} 个</span>
          <Button size="sm" variant="outline" onClick={() => batchMut.mutate({ action: "enable", ids: [...selected] })}>
            <CheckSquare className="size-4 mr-1" /> 启用
          </Button>
          <Button size="sm" variant="outline" onClick={() => batchMut.mutate({ action: "disable", ids: [...selected] })}>
            <XSquare className="size-4 mr-1" /> 禁用
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
                <TableHead>名称</TableHead>
                <TableHead>协议</TableHead>
                <TableHead>服务器</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>同步时间</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((node) => (
                <TableRow key={node.id}>
                  <TableCell><Checkbox checked={selected.has(node.id)} onCheckedChange={() => toggleSelect(node.id)} /></TableCell>
                  <TableCell className="font-medium max-w-[200px] truncate">{node.name}</TableCell>
                  <TableCell><Badge variant="outline">{node.protocol}</Badge></TableCell>
                  <TableCell className="text-muted-foreground text-sm">{node.server}:{node.port}</TableCell>
                  <TableCell><Badge variant={node.enabled ? "default" : "secondary"}>{node.enabled ? "启用" : "禁用"}</Badge></TableCell>
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
                <TableRow><TableCell colSpan={7} className="text-center text-muted-foreground py-8">暂无节点</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </Card>
      )}

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{editId ? "编辑节点" : "新建节点"}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2"><Label>名称</Label><Input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} /></div>
            <div className="space-y-2">
              <Label>协议</Label>
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
              <div className="col-span-2 space-y-2"><Label>服务器</Label><Input value={form.server} onChange={(e) => setForm((f) => ({ ...f, server: e.target.value }))} /></div>
              <div className="space-y-2"><Label>端口</Label><Input type="number" value={form.port} onChange={(e) => setForm((f) => ({ ...f, port: Number(e.target.value) }))} /></div>
            </div>
            <div className="space-y-2"><Label>配置 (JSON)</Label><Textarea value={form.config} onChange={(e) => setForm((f) => ({ ...f, config: e.target.value }))} rows={6} className="font-mono text-xs" /></div>
            <div className="space-y-2"><Label>标签（逗号分隔）</Label><Input value={form.tags} onChange={(e) => setForm((f) => ({ ...f, tags: e.target.value }))} /></div>
            <div className="flex items-center justify-between"><Label>启用</Label><Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} /></div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button onClick={handleSave} disabled={saveMut.isPending}>
              {saveMut.isPending && <Loader2 className="size-4 mr-1.5 animate-spin" />}
              {editId ? "保存" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Import Dialog */}
      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>导入节点</DialogTitle></DialogHeader>
          <div className="space-y-2">
            <Label>粘贴分享链接（每行一条）</Label>
            <Textarea value={importText} onChange={(e) => setImportText(e.target.value)} rows={8} placeholder="ss://...\nvmess://...\ntrojan://..." className="font-mono text-xs" />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportOpen(false)}>取消</Button>
            <Button onClick={() => importMut.mutate(importText)} disabled={importMut.isPending || !importText.trim()}>
              {importMut.isPending && <Loader2 className="size-4 mr-1.5 animate-spin" />} 导入
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <Dialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>删除节点</DialogTitle></DialogHeader>
          <p className="text-sm text-muted-foreground">此操作不可撤销。</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>取消</Button>
            <Button variant="destructive" onClick={() => deleteId && deleteMut.mutate(deleteId)} disabled={deleteMut.isPending}>删除</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
