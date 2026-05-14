import { useState, useCallback } from "react";
import { Outlet, Link, useLocation } from "react-router";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { post } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  LayoutDashboard,
  Rss,
  Server,
  BookOpen,
  FileCode2,
  ShieldCheck,
  ScrollText,
  CloudUpload,
  RefreshCw,
  PanelLeft,
  X,
} from "lucide-react";

interface NavItem {
  label: string;
  href: string;
  icon: React.ElementType;
}

const navItems: NavItem[] = [
  { label: "Dashboard", href: "/dashboard", icon: LayoutDashboard },
  { label: "Subscriptions", href: "/subscriptions", icon: Rss },
  { label: "Nodes", href: "/nodes", icon: Server },
  { label: "Rule Sources", href: "/rule-sources", icon: BookOpen },
  { label: "Templates", href: "/templates", icon: FileCode2 },
  { label: "Config Policies", href: "/configs", icon: ShieldCheck },
  { label: "Access Logs", href: "/config-access-logs", icon: ScrollText },
  { label: "DB Backup", href: "/backup", icon: CloudUpload },
];

export default function AppShell() {
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const handleRefreshCache = useCallback(async () => {
    setRefreshing(true);
    try {
      await post("/api/cache/policies/clear");
      toast.success("Cache cleared successfully");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to clear cache");
    } finally {
      setRefreshing(false);
    }
  }, []);

  const isActive = (href: string) => {
    if (href === "/dashboard") {
      return location.pathname === "/" || location.pathname === "/dashboard";
    }
    return location.pathname.startsWith(href);
  };

  return (
    <TooltipProvider>
      <div className="flex h-screen overflow-hidden bg-background">
        {/* Mobile overlay */}
        {sidebarOpen && (
          <div
            className="fixed inset-0 z-40 bg-black/60 backdrop-blur-xs lg:hidden"
            onClick={() => setSidebarOpen(false)}
          />
        )}

        {/* Sidebar */}
        <aside
          className={cn(
            "fixed inset-y-0 left-0 z-50 flex w-60 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-transform duration-200 lg:static lg:translate-x-0",
            sidebarOpen ? "translate-x-0" : "-translate-x-full"
          )}
        >
          {/* Logo / brand */}
          <div className="flex h-14 items-center justify-between px-4">
            <Link
              to="/dashboard"
              className="flex items-center gap-2.5 font-heading text-base font-semibold tracking-tight text-sidebar-foreground"
            >
              <div className="flex size-7 items-center justify-center rounded-md bg-primary text-primary-foreground">
                <svg viewBox="0 0 20 20" fill="none" className="size-4">
                  <circle cx="6" cy="5" r="2" fill="currentColor"/>
                  <path d="M6 7v3q0 2 2 3l4 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                  <path d="M6 10q0 2-1.5 3L3 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.5"/>
                  <path d="M8 5h5q2 0 3 2l1.5 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                  <circle cx="13" cy="16" r="1.6" fill="currentColor"/>
                  <circle cx="3" cy="15.5" r="1.3" fill="currentColor" opacity="0.5"/>
                  <circle cx="18" cy="11" r="1.6" fill="currentColor"/>
                </svg>
              </div>
              RuleFlow
            </Link>
            <Button
              variant="ghost"
              size="icon-sm"
              className="lg:hidden text-sidebar-foreground"
              onClick={() => setSidebarOpen(false)}
            >
              <X className="size-4" />
              <span className="sr-only">Close sidebar</span>
            </Button>
          </div>

          <Separator className="bg-sidebar-border" />

          {/* Navigation */}
          <ScrollArea className="flex-1 px-3 py-3">
            <div className="mb-2 px-2 text-[0.65rem] font-semibold uppercase tracking-widest text-sidebar-foreground/50">
              Workspace
            </div>
            <nav className="flex flex-col gap-0.5">
              {navItems.map((item) => {
                const active = isActive(item.href);
                const Icon = item.icon;
                return (
                  <Tooltip key={item.href}>
                    <TooltipTrigger
                      render={
                        <Link
                          to={item.href}
                          onClick={() => setSidebarOpen(false)}
                          className={cn(
                            "group flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-sm font-medium transition-colors",
                            active
                              ? "bg-sidebar-accent text-sidebar-accent-foreground"
                              : "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground"
                          )}
                        />
                      }
                    >
                      <Icon
                        className={cn(
                          "size-4 shrink-0",
                          active
                            ? "text-sidebar-primary"
                            : "text-sidebar-foreground/50 group-hover:text-sidebar-foreground/70"
                        )}
                      />
                      {item.label}
                    </TooltipTrigger>
                    <TooltipContent side="right" className="lg:hidden">
                      {item.label}
                    </TooltipContent>
                  </Tooltip>
                );
              })}
            </nav>
          </ScrollArea>

          <Separator className="bg-sidebar-border" />

          {/* Sidebar footer */}
          <div className="p-3">
            <Button
              variant="outline"
              size="sm"
              className="w-full justify-start gap-2 border-sidebar-border text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
              onClick={handleRefreshCache}
              disabled={refreshing}
            >
              <RefreshCw
                className={cn("size-3.5", refreshing && "animate-spin")}
              />
              {refreshing ? "Clearing…" : "Refresh Cache"}
            </Button>
          </div>
        </aside>

        {/* Main content area */}
        <div className="flex flex-1 flex-col overflow-hidden">
          {/* Top bar (mobile) */}
          <header className="flex h-14 items-center gap-3 border-b border-border px-4 lg:hidden">
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setSidebarOpen(true)}
            >
              <PanelLeft className="size-4" />
              <span className="sr-only">Open sidebar</span>
            </Button>
            <span className="font-heading text-sm font-semibold tracking-tight">
              RuleFlow
            </span>
          </header>

          {/* Page content */}
          <main className="flex-1 overflow-y-auto">
            <Outlet />
          </main>
        </div>

        <Toaster position="bottom-right" richColors closeButton />
      </div>
    </TooltipProvider>
  );
}
