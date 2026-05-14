import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, put, del } from "@/lib/api";
import type { Template } from "@/types";
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
import { Plus, Pencil, Trash2, Loader2, CheckCircle2, FileCode2, Globe, GlobeLock, ExternalLink } from "lucide-react";
import CodeEditor from "@/components/shared/code-editor";

const BASE = "https://github.com/0xUnixIO/RuleFlow/blob/main/template";
const TARGETS = [
  { label: "Clash Mihomo", value: "clash-mihomo", example: `${BASE}/clash.yaml` },
  { label: "Stash",        value: "stash",        example: `${BASE}/clash.yaml` },
  { label: "Surge",        value: "surge",        example: `${BASE}/surge.conf` },
  { label: "Sing-Box",     value: "sing-box",     example: `${BASE}/sing-box.json` },
];

const emptyForm = { name: "", description: "", content: "", target: "clash-mihomo", is_public: false, tags: "" };

function editorLang(target: string): "yaml" | "json" | "json5" {
  return target === "sing-box" ? "json5" : "yaml";
}

export default function TemplatesPage() {
  const qc = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState(emptyForm);
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [validating, setValidating] = useState(false);
  const [validResult, setValidResult] = useState<string | null>(null);

  const { data: templates, isLoading } = useQuery({
    queryKey: ["templates"],
    queryFn: () => get<Template[]>("/api/templates"),
  });

  const saveMut = useMutation({
    mutationFn: (data: Record<string, unknown>) =>
      editId ? put(`/api/templates/${editId}`, data) : post("/api/templates", data),
    onSuccess: () => {
      toast.success(editId ? "已更新" : "已创建");
      qc.invalidateQueries({ queryKey: ["templates"] });
      setDialogOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => del(`/api/templates/${id}`),
    onSuccess: () => {
      toast.success("已删除");
      qc.invalidateQueries({ queryKey: ["templates"] });
      setDeleteId(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const togglePublicMut = useMutation({
    mutationFn: (t: Template) => put(`/api/templates/${t.id}`, { ...t, is_public: !t.is_public }),
    onSuccess: (_data, t) => {
      toast.success(t.is_public ? "已设为私有" : "已设为公开");
      qc.invalidateQueries({ queryKey: ["templates"] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  function openCreate() {
    setEditId(null);
    setForm(emptyForm);
    setValidResult(null);
    setDialogOpen(true);
  }

  function openEdit(t: Template) {
    setEditId(t.id);
    setForm({ name: t.name, description: t.description, content: t.content, target: t.target, is_public: t.is_public, tags: (t.tags || []).join(", ") });
    setValidResult(null);
    setDialogOpen(true);
  }

  function handleSave() {
    saveMut.mutate({
      name: form.name, description: form.description, content: form.content,
      target: form.target, is_public: form.is_public,
      tags: form.tags.split(",").map((s) => s.trim()).filter(Boolean),
    });
  }

  async function handleValidate() {
    setValidating(true);
    setValidResult(null);
    try {
      await post("/api/templates/validate", { content: form.content, target: form.target });
      setValidResult("✅ 模板格式有效");
    } catch (e: unknown) {
      setValidResult(`❌ ${e instanceof Error ? e.message : "校验失败"}`);
    } finally {
      setValidating(false);
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">模板</h1>
          <p className="text-sm text-muted-foreground">配置模板管理</p>
        </div>
        <Button size="sm" onClick={openCreate}><Plus className="size-4 mr-1.5" /> 新建</Button>
      </div>

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-36 rounded-xl" />)}</div>
      ) : !templates?.length ? (
        <Card><CardContent className="py-12 text-center text-muted-foreground">暂无模板。</CardContent></Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {templates.map((t) => (
            <Card key={t.id}>
              <CardContent className="pt-4 space-y-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <FileCode2 className="size-4 text-muted-foreground shrink-0" />
                      <span className="font-medium truncate">{t.name}</span>
                    </div>
                    {t.description && <p className="text-xs text-muted-foreground mt-1 line-clamp-2">{t.description}</p>}
                  </div>
                  <div className="flex gap-1.5 shrink-0">
                    <Badge variant="outline">{t.target}</Badge>
                    {t.is_public && <Badge variant="secondary">公开</Badge>}
                  </div>
                </div>
                <div className="bg-muted rounded-md p-2 max-h-20 overflow-hidden">
                  <pre className="text-[10px] text-muted-foreground whitespace-pre-wrap break-all">{t.content.slice(0, 200)}{t.content.length > 200 ? "..." : ""}</pre>
                </div>
                <div className="flex gap-1.5 pt-1">
                  <Button
                    variant="ghost" size="sm"
                    className={`h-7 px-2 text-xs ${t.is_public ? "text-emerald-500 hover:text-emerald-600" : ""}`}
                    onClick={() => togglePublicMut.mutate(t)}
                    title={t.is_public ? "设为私有" : "设为公开"}
                  >
                    {t.is_public ? <Globe className="size-3 mr-1" /> : <GlobeLock className="size-3 mr-1" />}
                    {t.is_public ? "公开" : "私有"}
                  </Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => openEdit(t)}>
                    <Pencil className="size-3 mr-1" /> 编辑
                  </Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-destructive hover:text-destructive" onClick={() => setDeleteId(t.id)}>
                    <Trash2 className="size-3 mr-1" /> 删除
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-4xl w-[90vw] max-h-[90vh] overflow-hidden flex flex-col">
          <DialogHeader><DialogTitle>{editId ? "编辑模板" : "新建模板"}</DialogTitle></DialogHeader>
          <div className="flex-1 min-h-0 overflow-y-auto space-y-4 pr-1">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2"><Label>名称</Label><Input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} /></div>
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>目标格式</Label>
                {(() => {
                  const url = TARGETS.find((t) => t.value === form.target)?.example;
                  return url ? (
                    <a href={url} target="_blank" rel="noreferrer" className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
                      参考模板 <ExternalLink className="size-3" />
                    </a>
                  ) : null;
                })()}
              </div>
              <Select value={form.target} onValueChange={(v) => setForm((f) => ({ ...f, target: v }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{TARGETS.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2"><Label>描述</Label><Input value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} /></div>
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>内容</Label>
              <Button variant="outline" size="sm" className="h-7" onClick={handleValidate} disabled={validating || !form.content}>
                {validating ? <Loader2 className="size-3 mr-1 animate-spin" /> : <CheckCircle2 className="size-3 mr-1" />} 校验
              </Button>
            </div>
            <CodeEditor
              value={form.content}
              onChange={(v) => setForm((f) => ({ ...f, content: v }))}
              language={editorLang(form.target)}
              placeholder="YAML / JSON 模板内容…"
              className="h-96"
            />
            {validResult && <p className={`text-xs ${validResult.startsWith("✅") ? "text-emerald-500" : "text-destructive"}`}>{validResult}</p>}
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2 flex-1"><Label className="shrink-0">标签</Label><Input value={form.tags} onChange={(e) => setForm((f) => ({ ...f, tags: e.target.value }))} placeholder="逗号分隔" /></div>
            <div className="flex items-center gap-2"><Label>公开</Label><Switch checked={form.is_public} onCheckedChange={(v) => setForm((f) => ({ ...f, is_public: v }))} /></div>
          </div>
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

      <Dialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>删除模板</DialogTitle></DialogHeader>
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
