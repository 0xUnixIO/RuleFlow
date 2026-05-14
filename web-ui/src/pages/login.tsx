import { useState } from "react";
import { useSearchParams } from "react-router";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Eye, EyeOff, AlertCircle } from "lucide-react";

export default function LoginPage() {
  const [searchParams] = useSearchParams();
  const nextUrl = searchParams.get("next") || "/dashboard";
  const hasError = searchParams.get("error") === "1";

  const [showPassword, setShowPassword] = useState(false);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        {/* Branding */}
        <div className="mb-8 flex flex-col items-center gap-3">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-lg">
            <svg viewBox="0 0 20 20" fill="none" className="size-6">
              <circle cx="6" cy="5" r="2" fill="currentColor"/>
              <path d="M6 7v3q0 2 2 3l4 2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              <path d="M6 10q0 2-1.5 3L3 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" opacity="0.5"/>
              <path d="M8 5h5q2 0 3 2l1.5 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              <circle cx="13" cy="16" r="1.6" fill="currentColor"/>
              <circle cx="3" cy="15.5" r="1.3" fill="currentColor" opacity="0.5"/>
              <circle cx="18" cy="11" r="1.6" fill="currentColor"/>
            </svg>
          </div>
          <div className="text-center">
            <h1 className="font-heading text-2xl font-bold tracking-tight text-foreground">
              RuleFlow
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Proxy subscription management
            </p>
          </div>
        </div>

        {/* Login card */}
        <Card>
          <CardHeader>
            <CardTitle>Sign In</CardTitle>
            <CardDescription>
              Enter your password to access the dashboard.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {hasError && (
              <div className="mb-4 flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2.5 text-sm text-destructive">
                <AlertCircle className="size-4 shrink-0" />
                <span>Invalid password. Please try again.</span>
              </div>
            )}

            <form method="POST" action="/login" className="flex flex-col gap-4">
              <input type="hidden" name="next" value={nextUrl} />

              <div className="flex flex-col gap-2">
                <Label htmlFor="password">Password</Label>
                <div className="relative">
                  <Input
                    id="password"
                    name="password"
                    type={showPassword ? "text" : "password"}
                    placeholder="Enter your password"
                    required
                    autoFocus
                    autoComplete="current-password"
                    className="pr-9"
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute inset-y-0 right-0 flex items-center justify-center w-9 text-muted-foreground hover:text-foreground transition-colors"
                    tabIndex={-1}
                    aria-label={showPassword ? "Hide password" : "Show password"}
                  >
                    {showPassword ? (
                      <EyeOff className="size-4" />
                    ) : (
                      <Eye className="size-4" />
                    )}
                  </button>
                </div>
              </div>

              <Button type="submit" className="w-full">
                Sign In
              </Button>
            </form>
          </CardContent>
        </Card>

        <p className="mt-6 text-center text-xs text-muted-foreground/60">
          Session secured via HTTP-only cookie
        </p>
      </div>
    </div>
  );
}
