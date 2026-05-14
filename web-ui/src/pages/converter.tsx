import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Copy, ExternalLink, Shield } from "lucide-react";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";

interface PublicTemplate { id: number; name: string; target: string; content?: string; }

const TARGETS = [
  { label: "Clash Mihomo", value: "clash-mihomo" },
  { label: "Stash", value: "stash" },
  { label: "Surge", value: "surge" },
  { label: "Sing-Box", value: "sing-box" },
];

export default function ConverterPage() {
  const [subUrl, setSubUrl] = useState("");
  const [target, setTarget] = useState("clash-mihomo");
  const [templateId, setTemplateId] = useState("");
  const [templates, setTemplates] = useState<PublicTemplate[]>([]);
  const [templateContent, setTemplateContent] = useState("");
  const [preview, setPreview] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetch("/api/templates/public").then((r) => r.json()).then((d) => {
      if (d.success) setTemplates(d.data || []);
    }).catch(() => {});
  }, []);

  const filteredTemplates = templates.filter((t) => t.target === target);

  useEffect(() => {
    if (!templateId) { setTemplateContent(""); return; }
    fetch(`/api/templates/public/${templateId}`).then((r) => r.json()).then((d) => {
      if (d.success) setTemplateContent(d.data?.content || "");
    }).catch(() => {});
  }, [templateId]);

  const convertUrl = subUrl
    ? `${window.location.origin}/convert?url=${encodeURIComponent(subUrl)}&target=${target}${templateId ? `&template=${templateId}` : ""}`
    : "";

  const fetchPreview = useCallback(async () => {
    if (!subUrl) return;
    setLoading(true);
    try {
      const res = await fetch(`/convert?url=${encodeURIComponent(subUrl)}&target=${target}${templateId ? `&template=${templateId}` : ""}`);
      setPreview(await res.text());
    } catch { setPreview("Failed to fetch preview"); }
    finally { setLoading(false); }
  }, [subUrl, target, templateId]);

  useEffect(() => {
    if (!subUrl) { setPreview(""); return; }
    const timer = setTimeout(fetchPreview, 800);
    return () => clearTimeout(timer);
  }, [subUrl, target, templateId, fetchPreview]);

  async function copyUrl() {
    if (!convertUrl) return;
    await navigator.clipboard.writeText(convertUrl);
    toast.success("URL copied");
  }

  return (
    <div className="min-h-screen bg-background">
      <Toaster position="bottom-right" richColors closeButton />
      <div className="mx-auto max-w-4xl p-6 space-y-6">
        <div className="flex items-center gap-3">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Shield className="size-5" />
          </div>
          <div>
            <h1 className="font-heading text-2xl font-bold tracking-tight">Subscription Converter</h1>
            <p className="text-sm text-muted-foreground">Convert subscription links to different formats</p>
          </div>
        </div>

        <Card>
          <CardContent className="pt-6 space-y-4">
            <div className="space-y-2">
              <Label>Subscription URL</Label>
              <Input value={subUrl} onChange={(e) => setSubUrl(e.target.value)} placeholder="https://your-subscription-url..." />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label>Output Format</Label>
                <Select value={target} onValueChange={(v) => { setTarget(v); setTemplateId(""); }}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>{TARGETS.map((t) => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}</SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Template (optional)</Label>
                <Select value={templateId} onValueChange={setTemplateId}>
                  <SelectTrigger><SelectValue placeholder="Default" /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Default (no template)</SelectItem>
                    {filteredTemplates.map((t) => <SelectItem key={t.id} value={String(t.id)}>{t.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
            </div>
            {convertUrl && (
              <div className="space-y-2">
                <Label>Generated URL</Label>
                <div className="flex gap-2">
                  <Input readOnly value={convertUrl} className="font-mono text-xs" />
                  <Button variant="outline" size="sm" onClick={copyUrl}><Copy className="size-4" /></Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <Tabs defaultValue="preview">
          <TabsList>
            <TabsTrigger value="preview">Conversion Preview</TabsTrigger>
            <TabsTrigger value="template">Template Content</TabsTrigger>
          </TabsList>
          <TabsContent value="preview">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Preview</CardTitle>
                {loading && <Badge variant="outline">Loading...</Badge>}
              </CardHeader>
              <CardContent>
                <pre className="max-h-96 overflow-auto rounded-md bg-muted p-3 text-xs font-mono whitespace-pre-wrap">
                  {preview || "Enter a subscription URL to see preview"}
                </pre>
              </CardContent>
            </Card>
          </TabsContent>
          <TabsContent value="template">
            <Card>
              <CardHeader><CardTitle className="text-base">Template Content</CardTitle></CardHeader>
              <CardContent>
                <pre className="max-h-96 overflow-auto rounded-md bg-muted p-3 text-xs font-mono whitespace-pre-wrap">
                  {templateContent || "Select a template to view its content"}
                </pre>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        <div className="text-center text-xs text-muted-foreground">
          Powered by <a href="/" className="inline-flex items-center gap-1 text-primary hover:underline">RuleFlow <ExternalLink className="size-3" /></a>
        </div>
      </div>
    </div>
  );
}
