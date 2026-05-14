import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, put, post } from "@/lib/api";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { CloudUpload, Play, Plug, Loader2, ShieldCheck, RefreshCw, RotateCcw, Terminal, Trash2 } from "lucide-react";
import type { BackupSettings, R2Object, SqlResult } from "@/types";

export default function BackupPage() {
  const qc = useQueryClient();

  const { data: settings, isLoading: loadingSettings } = useQuery({
    queryKey: ["backupSettings"],
    queryFn: () => get<BackupSettings>("/api/backup/settings"),
  });

  const { data: r2Objects, isLoading: loadingR2, refetch: refetchR2 } = useQuery({
    queryKey: ["r2Objects"],
    queryFn: () => get<R2Object[]>("/api/backup/r2-objects"),
    retry: false,
  });

  const [form, setForm] = useState<Partial<BackupSettings>>({});
  const merged = { ...settings, ...form } as BackupSettings;

  function field(key: keyof BackupSettings) {
    return {
      value: (form[key] ?? settings?.[key] ?? "") as string,
      onChange: (e: React.ChangeEvent<HTMLInputElement>) =>
        setForm((f) => ({ ...f, [key]: e.target.value })),
    };
  }

  const saveMut = useMutation({
    mutationFn: () => put("/api/backup/settings", merged),
    onSuccess: () => {
      toast.success("配置已保存");
      qc.invalidateQueries({ queryKey: ["backupSettings"] });
      setForm({});
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const [testing, setTesting] = useState(false);
  async function handleTest() {
    setTesting(true);
    try {
      await post("/api/backup/test", {
        r2_account_id: merged.r2_account_id,
        r2_access_key_id: merged.r2_access_key_id,
        r2_secret_access_key: merged.r2_secret_access_key,
        r2_bucket_name: merged.r2_bucket_name,
      });
      toast.success("R2 连接成功");
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "连接失败");
    } finally {
      setTesting(false);
    }
  }

  const triggerMut = useMutation({
    mutationFn: () => post("/api/backup/trigger"),
    onSuccess: () => {
      toast.success("备份已完成");
      refetchR2();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  // 恢复确认弹窗
  const [restoreTarget, setRestoreTarget] = useState<string | null>(null);

  // SQL Executor
  const [sql, setSql] = useState("");
  const [sqlRunning, setSqlRunning] = useState(false);
  const [sqlResult, setSqlResult] = useState<SqlResult | null>(null);

  async function handleSql() {
    if (!sql.trim()) return;
    setSqlRunning(true);
    setSqlResult(null);
    try {
      const data = await post<SqlResult>("/api/admin/exec-sql", { sql: sql.trim() });
      setSqlResult(data);
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "SQL failed");
    } finally {
      setSqlRunning(false);
    }
  }
  const restoreMut = useMutation({
    mutationFn: (fileKey: string) => post("/api/backup/restore", { file_key: fileKey }),
    onSuccess: () => {
      toast.success("数据库已从备份恢复");
      setRestoreTarget(null);
    },
    onError: (e: Error) => {
      toast.error(e.message);
      setRestoreTarget(null);
    },
  });

  function formatBytes(bytes: number) {
    if (bytes === 0) return "—";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
  }

  function formatDate(s: string) {
    return new Date(s).toLocaleString("zh-CN", { hour12: false });
  }

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div>
        <h1 className="text-xl font-semibold">数据库备份</h1>
        <p className="text-sm text-muted-foreground mt-1">
          每 6 小时自动备份所有表到 Cloudflare R2，每张表一个 CSV，打包为 tar.gz，保留最近 6 份
        </p>
      </div>

      {/* R2 配置 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <ShieldCheck className="size-4" />
            Cloudflare R2 配置
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {loadingSettings ? (
            <div className="space-y-3">
              {[...Array(4)].map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between">
                <div>
                  <Label>启用自动备份</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">关闭后定时任务不执行，手动触发仍可用</p>
                </div>
                <Switch
                  checked={form.enabled ?? settings?.enabled ?? false}
                  onCheckedChange={(v) => setForm((f) => ({ ...f, enabled: v }))}
                />
              </div>

              <Separator />

              <div className="grid gap-3">
                <div className="space-y-1.5">
                  <Label>Account ID</Label>
                  <Input placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" {...field("r2_account_id")} />
                </div>
                <div className="space-y-1.5">
                  <Label>Access Key ID</Label>
                  <Input placeholder="Access Key ID" {...field("r2_access_key_id")} />
                </div>
                <div className="space-y-1.5">
                  <Label>Secret Access Key</Label>
                  <Input
                    type="password"
                    placeholder={settings?.r2_secret_access_key ? "••••••••（已设置）" : "Secret Access Key"}
                    {...field("r2_secret_access_key")}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>Bucket Name</Label>
                  <Input placeholder="my-backup-bucket" {...field("r2_bucket_name")} />
                </div>
              </div>

              <div className="flex items-center gap-2 pt-1">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleTest}
                  disabled={testing || !merged.r2_account_id}
                >
                  {testing ? <Loader2 className="size-3.5 mr-1.5 animate-spin" /> : <Plug className="size-3.5 mr-1.5" />}
                  测试连接
                </Button>
                <Button size="sm" onClick={() => saveMut.mutate()} disabled={saveMut.isPending}>
                  {saveMut.isPending && <Loader2 className="size-3.5 mr-1.5 animate-spin" />}
                  保存配置
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* 手动触发 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <CloudUpload className="size-4" />
            手动备份
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground mb-3">
            立即执行一次备份，不受启用开关限制。
          </p>
          <Button onClick={() => triggerMut.mutate()} disabled={triggerMut.isPending}>
            {triggerMut.isPending
              ? <Loader2 className="size-4 mr-2 animate-spin" />
              : <Play className="size-4 mr-2" />}
            {triggerMut.isPending ? "备份中…" : "立即备份"}
          </Button>
        </CardContent>
      </Card>

      {/* R2 文件列表（从 R2 直接查询） */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">R2 备份文件</CardTitle>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetchR2()}
              disabled={loadingR2}
            >
              {loadingR2 ? <Loader2 className="size-3.5 mr-1.5 animate-spin" /> : <RefreshCw className="size-3.5 mr-1.5" />}
              刷新
            </Button>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {loadingR2 ? (
            <div className="p-4 space-y-2">
              {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-10 w-full" />)}
            </div>
          ) : !r2Objects?.length ? (
            <p className="text-sm text-muted-foreground text-center py-8">R2 中暂无备份文件</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>文件名</TableHead>
                  <TableHead>大小</TableHead>
                  <TableHead>上传时间</TableHead>
                  <TableHead className="w-20" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {r2Objects.map((obj) => (
                  <TableRow key={obj.key}>
                    <TableCell className="font-mono text-xs max-w-[220px] truncate" title={obj.key}>
                      {obj.key}
                    </TableCell>
                    <TableCell className="text-sm">{formatBytes(obj.size)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">
                      {formatDate(obj.last_modified)}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-xs h-7"
                        onClick={() => setRestoreTarget(obj.key)}
                      >
                        <RotateCcw className="size-3 mr-1" />
                        恢复
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* SQL Executor */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <Terminal className="size-4" />
            SQL Executor
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1.5">
            <Label>SQL Query</Label>
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
            <Button onClick={handleSql} disabled={sqlRunning || !sql.trim()} size="sm">
              {sqlRunning ? <Loader2 className="size-3.5 mr-1.5 animate-spin" /> : <Play className="size-3.5 mr-1.5" />}
              Run
            </Button>
            <Button variant="outline" size="sm" onClick={() => { setSql(""); setSqlResult(null); }}>
              <Trash2 className="size-3.5 mr-1.5" /> Clear
            </Button>
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
                  {sqlResult.type === "select" ? "No rows returned" : `${sqlResult.rows_affected} row(s) affected`}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 恢复确认弹窗 */}
      <Dialog open={!!restoreTarget} onOpenChange={(open) => !open && setRestoreTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认恢复数据库？</DialogTitle>
          </DialogHeader>
          <div className="text-sm text-muted-foreground space-y-2">
            <p>即将从以下备份恢复数据库：</p>
            <p className="font-mono text-xs bg-muted rounded px-2 py-1.5 break-all">{restoreTarget}</p>
            <p className="text-destructive font-medium">
              ⚠️ 此操作将清空当前所有数据并替换为备份内容，不可撤销。
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRestoreTarget(null)}>取消</Button>
            <Button
              variant="destructive"
              onClick={() => restoreTarget && restoreMut.mutate(restoreTarget)}
              disabled={restoreMut.isPending}
            >
              {restoreMut.isPending && <Loader2 className="size-4 mr-2 animate-spin" />}
              确认恢复
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
