import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, put, del } from "@/lib/api";
import type { ConfigPolicy, Subscription, Template, Node } from "@/types";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Plus, Pencil, Trash2, Copy, RefreshCw, Loader2, Shield, Link2 } from "lucide-react";

const TARGETS = [
  { label: "Clash Mihomo", value: "clash-mihomo" },
  { label: "Stash", value: "stash" },
  { label: "Surge", value: "surge" },
  { label: "Sing-Box", value: "sing-box" },
];

const emptyForm = { name: "", description: "", target: "clash-mihomo", template_name: "", enabled: true, subscription_ids: [] as number[], node_ids: [] as number[] };

export default function ConfigsPage() {
  const qc = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [deleteId, setDeleteId] = useState<number | null>(null);

  const { data: policies, isLoading } = useQuery({ queryKey: ["configPolicies"], queryFn: () => get<ConfigPolicy[]>("/api/config-policies") });
  const { data: subs } = useQuery({ queryKey: ["subscriptions"], queryFn: () => get<Subscription[]>("/api/subscriptions") });
  const { data: templates } = useQuery({ queryKey: ["templates"], queryFn: () => get<Template[]>("/api/templates") });
  const { data: nodes } = useQuery({ queryKey: ["nodes"], queryFn: () => get<Node[]>("/api/nodes") });

  const manualNodes = nodes?.filter((n) => !n.source_id) ?? [];

  const saveMut = useMutation({
    mutationFn: (data: typeof form) =>
      editId ? put(`/api/config-policies/${editId}`, data) : post("/api/config-policies", data),
    onSuccess: () => {
      toast.success(editId ? "已更新" : "已创建");
      qc.invalidateQueries({ queryKey: ["configPolicies"] });
      setDialogOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => del(`/api/config-policies/${id}`),
    onSuccess: () => {
      toast.success("已删除");
      qc.invalidateQueries({ queryKey: ["configPolicies"] });
      setDeleteId(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  function openCreate() { setEditId(null); setForm(emptyForm); setDialogOpen(true); }
  function openEdit(p: ConfigPolicy) {
    setEditId(p.id);
    setForm({
      name: p.name, description: p.description, target: p.target,
      template_name: p.template_name, enabled: p.enabled,
      subscription_ids: p.subscription_ids || [], node_ids: p.node_ids || [],
    });
    setDialogOpen(true);
  }

  async function copySubscribeUrl(token: string) {
    const url = `${window.location.origin}/subscribe?token=${token}`;
    await navigator.clipboard.writeText(url);
    toast.success("订阅 URL 已复制");
  }

  async function clearCache(id: number) {
    try {
      await del(`/api/config-policies/${id}/cache`);
      toast.success("Cache 已清除");
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : "操作失败"); }
  }

  function toggleSubId(id: number) {
    setForm((f) => ({
      ...f,
      subscription_ids: f.subscription_ids.includes(id)
        ? f.subscription_ids.filter((x) => x !== id)
        : [...f.subscription_ids, id],
    }));
  }

  function toggleNodeId(id: number) {
    setForm((f) => ({
      ...f,
      node_ids: f.node_ids.includes(id)
        ? f.node_ids.filter((x) => x !== id)
        : [...f.node_ids, id],
    }));
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">配置策略</h1>
          <p className="text-sm text-muted-foreground">管理配置下发策略</p>
        </div>
        <Button size="sm" onClick={openCreate}><Plus className="size-4 mr-1.5" /> 新建</Button>
      </div>

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2">{[1, 2].map((i) => <Skeleton key={i} className="h-40 rounded-xl" />)}</div>
      ) : !policies?.length ? (
        <Card><CardContent className="py-12 text-center text-muted-foreground">暂无配置策略。</CardContent></Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {policies.map((p) => (
            <Card key={p.id}>
              <CardContent className="pt-4 space-y-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <Shield className="size-4 text-muted-foreground shrink-0" />
                      <span className="font-medium truncate">{p.name}</span>
                    </div>
                    {p.description && <p className="text-xs text-muted-foreground mt-1">{p.description}</p>}
                  </div>
                  <div className="flex gap-1.5 shrink-0">
                    <Badge variant="outline">{p.target}</Badge>
                    <Badge variant={p.enabled ? "default" : "secondary"}>{p.enabled ? "启用" : "禁用"}</Badge>
                  </div>
                </div>
                <div className="flex flex-wrap gap-1.5 text-xs">
                  <span className="text-muted-foreground">模板：{p.template_name || "—"}</span>
                  <span className="text-muted-foreground">• 订阅：{p.subscription_ids?.length || 0}</span>
                  <span className="text-muted-foreground">• 节点：{p.node_ids?.length || 0}</span>
                </div>
                <div className="flex items-center gap-1.5 text-xs bg-muted rounded-md px-2 py-1.5">
                  <Link2 className="size-3 text-muted-foreground shrink-0" />
                  <code className="truncate text-[10px]">/subscribe?token={p.token}</code>
                  <Button variant="ghost" size="sm" className="h-5 px-1 ml-auto" onClick={() => copySubscribeUrl(p.token)}><Copy className="size-3" /></Button>
                </div>
                <div className="flex gap-1.5 pt-1">
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => openEdit(p)}><Pencil className="size-3 mr-1" /> 编辑</Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => clearCache(p.id)}><RefreshCw className="size-3 mr-1" /> 清除 Cache</Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-destructive hover:text-destructive" onClick={() => setDeleteId(p.id)}><Trash2 className="size-3 mr-1" /> 删除</Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
          <DialogHeader><DialogTitle>{editId ? "编辑策略" : "新建策略"}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2"><Label>名称</Label><Input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} /></div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label>目标格式</Label>
                <Select value={form.target} onValueChange={(v) => setForm((f) => ({ ...f, target: v }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>{TARGETS.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>模板</Label>
                <Select value={form.template_name} onValueChange={(v) => setForm((f) => ({ ...f, template_name: v }))}>
                  <SelectTrigger><SelectValue placeholder="选择…" /></SelectTrigger>
                  <SelectContent>{templates?.map((t) => <SelectItem key={t.name} value={t.name}>{t.name}</SelectItem>)}</SelectContent>
                </Select>
              </div>
            </div>
            <div className="space-y-2"><Label>描述</Label><Textarea value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} rows={2} /></div>
            <div className="flex items-center justify-between"><Label>启用</Label><Switch checked={form.enabled} onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))} /></div>
            {subs && subs.length > 0 && (
              <div className="space-y-2">
                <Label>订阅源</Label>
                <div className="max-h-32 overflow-y-auto space-y-1.5 border rounded-md p-2">
                  {subs.map((s) => (
                    <label key={s.id} className="flex items-center gap-2 text-sm cursor-pointer">
                      <Checkbox checked={form.subscription_ids.includes(s.id)} onCheckedChange={() => toggleSubId(s.id)} />
                      {s.name}
                    </label>
                  ))}
                </div>
              </div>
            )}
            {manualNodes.length > 0 && (
              <div className="space-y-2">
                <Label>手动节点</Label>
                <div className="max-h-32 overflow-y-auto space-y-1.5 border rounded-md p-2">
                  {manualNodes.map((n) => (
                    <label key={n.id} className="flex items-center gap-2 text-sm cursor-pointer">
                      <Checkbox checked={form.node_ids.includes(n.id)} onCheckedChange={() => toggleNodeId(n.id)} />
                      {n.name} <Badge variant="outline" className="text-[10px]">{n.protocol}</Badge>
                    </label>
                  ))}
                </div>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button onClick={() => saveMut.mutate(form)} disabled={saveMut.isPending}>
              {saveMut.isPending && <Loader2 className="size-4 mr-1.5 animate-spin" />}{editId ? "保存" : "创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>删除策略</DialogTitle></DialogHeader>
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
