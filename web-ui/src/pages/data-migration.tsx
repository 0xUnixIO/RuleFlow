import { useState } from "react";
import { post } from "@/lib/api";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Separator } from "@/components/ui/separator";
import { Download, Play, Loader2, Trash2 } from "lucide-react";
import type { SqlResult } from "@/types";

export default function DataMigrationPage() {
  const [sql, setSql] = useState("");
  const [sqlRunning, setSqlRunning] = useState(false);
  const [sqlResult, setSqlResult] = useState<SqlResult | null>(null);

  async function handleExport() {
    try {
      const res = await fetch("/api/export", { credentials: "include" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `ruleflow-export-${new Date().toISOString().slice(0, 10)}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success("导出文件已下载");
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "导出失败");
    }
  }

  async function handleSql() {
    if (!sql.trim()) return;
    setSqlRunning(true);
    setSqlResult(null);
    try {
      const data = await post<SqlResult>("/api/admin/exec-sql", { sql: sql.trim() });
      setSqlResult(data);
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "SQL 执行失败");
    } finally {
      setSqlRunning(false);
    }
  }

  return (
    <div className="space-y-6 p-6">
      <div>
        <h1 className="font-heading text-2xl font-bold tracking-tight">数据迁移</h1>
        <p className="text-sm text-muted-foreground">导出配置数据并执行原始 SQL</p>
      </div>

      {/* 导出 */}
      <Card>
        <CardHeader><CardTitle className="text-base">导出数据</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">将所有数据下载为 JSON 文件。</p>
          <Button onClick={handleExport}><Download className="size-4 mr-1.5" /> 导出</Button>
        </CardContent>
      </Card>

      {/* SQL 执行器 */}
      <Card>
        <CardHeader><CardTitle className="text-base">SQL 执行器</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label>SQL 语句</Label>
            <Textarea
              value={sql}
              onChange={(e) => setSql(e.target.value)}
              rows={4}
              className="font-mono text-xs"
              placeholder="SELECT * FROM ..."
              onKeyDown={(e) => { if ((e.metaKey || e.ctrlKey) && e.key === "Enter") handleSql(); }}
            />
          </div>
          <div className="flex gap-2">
            <Button onClick={handleSql} disabled={sqlRunning || !sql.trim()}>
              {sqlRunning ? <Loader2 className="size-4 mr-1.5 animate-spin" /> : <Play className="size-4 mr-1.5" />} 执行
            </Button>
            <Button variant="outline" onClick={() => { setSql(""); setSqlResult(null); }}><Trash2 className="size-4 mr-1.5" /> 清空</Button>
          </div>
          {sqlResult && (
            <div className="space-y-2">
              <Separator />
              {sqlResult.type === "select" && sqlResult.rows?.length > 0 ? (
                <div className="max-h-96 overflow-auto rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>{sqlResult.columns.map((c) => <TableHead key={c} className="text-xs">{c}</TableHead>)}</TableRow>
                    </TableHeader>
                    <TableBody>
                      {sqlResult.rows.slice(0, 500).map((row, i) => (
                        <TableRow key={i}>
                          {sqlResult.columns.map((c) => (
                            <TableCell key={c} className="text-xs font-mono max-w-[200px] truncate">{String(row[c] ?? "NULL")}</TableCell>
                          ))}
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {sqlResult.type === "select" ? "无返回行" : `影响 ${sqlResult.rows_affected} 行`}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
