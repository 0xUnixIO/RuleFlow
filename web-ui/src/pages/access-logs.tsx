import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { get } from "@/lib/api";
import type { ConfigAccessLog, ConfigPolicy } from "@/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { RefreshCw } from "lucide-react";

function formatTime(d: string) {
  return new Date(d).toLocaleString("zh-CN", { hour12: false });
}

export default function AccessLogsPage() {
  const [filters, setFilters] = useState({ policy_id: "", success: "", cache_hit: "", keyword: "" });

  const { data: policies } = useQuery({ queryKey: ["configPolicies"], queryFn: () => get<ConfigPolicy[]>("/api/config-policies") });

  const queryParams = new URLSearchParams();
  if (filters.policy_id) queryParams.set("policy_id", filters.policy_id);
  if (filters.success) queryParams.set("success", filters.success);
  if (filters.cache_hit) queryParams.set("cache_hit", filters.cache_hit);
  if (filters.keyword) queryParams.set("keyword", filters.keyword);

  const { data: logs, isLoading, refetch } = useQuery({
    queryKey: ["accessLogs", filters],
    queryFn: () => get<ConfigAccessLog[]>(`/api/config-access-logs?${queryParams.toString()}`),
    refetchInterval: 30000,
  });

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-heading text-2xl font-bold tracking-tight">访问日志</h1>
          <p className="text-sm text-muted-foreground">配置访问历史记录</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()}><RefreshCw className="size-4 mr-1.5" /> 刷新</Button>
      </div>

      <div className="flex flex-wrap gap-3">
        <Select value={filters.policy_id || "all"} onValueChange={(v) => setFilters((f) => ({ ...f, policy_id: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-44"><SelectValue placeholder="策略" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部策略</SelectItem>
            {policies?.map((p) => <SelectItem key={p.id} value={String(p.id)}>{p.name}</SelectItem>)}
          </SelectContent>
        </Select>
        <Select value={filters.success || "all"} onValueChange={(v) => setFilters((f) => ({ ...f, success: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-32"><SelectValue placeholder="状态" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部</SelectItem>
            <SelectItem value="true">成功</SelectItem>
            <SelectItem value="false">失败</SelectItem>
          </SelectContent>
        </Select>
        <Select value={filters.cache_hit || "all"} onValueChange={(v) => setFilters((f) => ({ ...f, cache_hit: v === "all" ? "" : v }))}>
          <SelectTrigger className="w-32"><SelectValue placeholder="Cache" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部</SelectItem>
            <SelectItem value="true">Cache 命中</SelectItem>
            <SelectItem value="false">Cache 未命中</SelectItem>
          </SelectContent>
        </Select>
        <Input placeholder="搜索…" className="w-48" value={filters.keyword} onChange={(e) => setFilters((f) => ({ ...f, keyword: e.target.value }))} />
      </div>

      {isLoading ? (
        <Skeleton className="h-64 rounded-xl" />
      ) : (
        <Card className="overflow-hidden">
            <Table containerClassName="max-h-[calc(100vh-300px)] overflow-y-auto">
              <TableHeader className="sticky top-0 z-10 bg-card">
                <TableRow>
                  <TableHead>时间</TableHead>
                  <TableHead>Token</TableHead>
                  <TableHead>IP</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>Cache</TableHead>
                  <TableHead>节点数</TableHead>
                  <TableHead>User Agent</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs?.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="text-xs whitespace-nowrap">{formatTime(log.created_at)}</TableCell>
                    <TableCell className="font-mono text-xs max-w-[100px] truncate">{log.token}</TableCell>
                    <TableCell className="text-xs">{log.client_ip || "—"}</TableCell>
                    <TableCell>
                      <Badge variant={log.success ? "default" : "destructive"} className="text-xs">
                        {log.success ? `${log.status_code}` : `${log.status_code}`}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {log.cache_hit ? (
                        <Badge variant="outline" className="text-xs text-emerald-500">HIT</Badge>
                      ) : (
                        <Badge variant="outline" className="text-xs">MISS</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs">{log.node_count ?? "—"}</TableCell>
                    <TableCell className="text-xs max-w-[200px] truncate text-muted-foreground">{log.user_agent}</TableCell>
                  </TableRow>
                ))}
                {!logs?.length && (
                  <TableRow><TableCell colSpan={7} className="text-center text-muted-foreground py-8">暂无日志</TableCell></TableRow>
                )}
              </TableBody>
            </Table>
        </Card>
      )}
    </div>
  );
}
