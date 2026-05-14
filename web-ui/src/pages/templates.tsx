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
import { Plus, Pencil, Trash2, Loader2, CheckCircle2, FileCode2, Globe, GlobeLock } from "lucide-react";
import CodeEditor from "@/components/shared/code-editor";

const TARGETS = [
  { label: "Clash Mihomo", value: "clash-mihomo" },
  { label: "Stash", value: "stash" },
  { label: "Surge", value: "surge" },
  { label: "Sing-Box", value: "sing-box" },
];

const emptyForm = { name: "", description: "", content: "", target: "clash-mihomo", is_public: false, tags: "" };

function editorLang(target: string): "yaml" | "json" {
  return target === "sing-box" ? "json" : "yaml";
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
      toast.success(editId ? "Updated" : "Created");
      qc.invalidateQueries({ queryKey: ["templates"] });
      setDialogOpen(false);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => del(`/api/templates/${id}`),
    onSuccess: () => {
      toast.success("Deleted");
      qc.invalidateQueries({ queryKey: ["templates"] });
      setDeleteId(null);
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const togglePublicMut = useMutation({
    mutationFn: (t: Template) => put(`/api/templates/${t.id}`, { ...t, is_public: !t.is_public }),
    onSuccess: (_data, t) => {
      toast.success(t.is_public ? "Set to private" : "Set to public");
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
      setValidResult("✅ Template is valid");
    } catch (e: unknown) {
      setValidResult(`❌ ${e instanceof Error ? e.message : "Validation failed"}`);
    } finally {
      setValidating(false);
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">Templates</h1>
          <p className="text-sm text-muted-foreground">Configuration templates</p>
        </div>
        <Button size="sm" onClick={openCreate}><Plus className="size-4 mr-1.5" /> New</Button>
      </div>

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{[1, 2, 3].map((i) => <Skeleton key={i} className="h-36 rounded-xl" />)}</div>
      ) : !templates?.length ? (
        <Card><CardContent className="py-12 text-center text-muted-foreground">No templates yet.</CardContent></Card>
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
                    {t.is_public && <Badge variant="secondary">Public</Badge>}
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
                    title={t.is_public ? "Set to private" : "Set to public"}
                  >
                    {t.is_public ? <Globe className="size-3 mr-1" /> : <GlobeLock className="size-3 mr-1" />}
                    {t.is_public ? "Public" : "Private"}
                  </Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => openEdit(t)}>
                    <Pencil className="size-3 mr-1" /> Edit
                  </Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-destructive hover:text-destructive" onClick={() => setDeleteId(t.id)}>
                    <Trash2 className="size-3 mr-1" /> Delete
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-4xl w-[90vw] max-h-[90vh] overflow-hidden flex flex-col">
          <DialogHeader><DialogTitle>{editId ? "Edit Template" : "New Template"}</DialogTitle></DialogHeader>
          <div className="flex-1 min-h-0 overflow-y-auto space-y-4 pr-1">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2"><Label>Name</Label><Input value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} /></div>
            <div className="space-y-2">
              <Label>Target</Label>
              <Select value={form.target} onValueChange={(v) => setForm((f) => ({ ...f, target: v }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>{TARGETS.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2"><Label>Description</Label><Input value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} /></div>
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Content</Label>
              <Button variant="outline" size="sm" className="h-7" onClick={handleValidate} disabled={validating || !form.content}>
                {validating ? <Loader2 className="size-3 mr-1 animate-spin" /> : <CheckCircle2 className="size-3 mr-1" />} Validate
              </Button>
            </div>
            <CodeEditor
              value={form.content}
              onChange={(v) => setForm((f) => ({ ...f, content: v }))}
              language={editorLang(form.target)}
              placeholder="YAML / JSON template content..."
              className="h-96"
            />
            {validResult && <p className={`text-xs ${validResult.startsWith("✅") ? "text-emerald-500" : "text-destructive"}`}>{validResult}</p>}
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2 flex-1"><Label className="shrink-0">Tags</Label><Input value={form.tags} onChange={(e) => setForm((f) => ({ ...f, tags: e.target.value }))} placeholder="comma-separated" /></div>
            <div className="flex items-center gap-2"><Label>Public</Label><Switch checked={form.is_public} onCheckedChange={(v) => setForm((f) => ({ ...f, is_public: v }))} /></div>
          </div>
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

      <Dialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <DialogContent>
          <DialogHeader><DialogTitle>Delete Template</DialogTitle></DialogHeader>
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
