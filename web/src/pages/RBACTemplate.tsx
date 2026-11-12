import { type Component, createSignal, For, Show, createMemo } from 'solid-js';
import { useI18n } from '~/i18n';
import { useAuth, PermissionGuard } from '~/auth';
import {
  getRoles,
  getAllPermissions,
  getRolePermissions,
  getUsers,
  getMe,
} from '~/api';
import { PERMISSION_RESOURCES } from '~/domain';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '~/components/ui/card';
import { Button } from '~/components/ui/button';
import { Badge } from '~/components/ui/badge';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import {
  IconShield,
  IconShieldCheck,
  IconLock,
  IconCheck,
  IconAlertCircle,
  IconKey,
  IconCode,
  IconApiApp,
  IconSparkles,
  IconLoader2,
  IconEye,
  IconPlus,
  IconPencil,
  IconTrash,
} from '@tabler/icons-solidjs';

type Verb = 'read' | 'create' | 'update' | 'delete';
const ALL_VERBS: Verb[] = ['read', 'create', 'update', 'delete'];

function getErrorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (err && typeof err === 'object' && 'message' in err && typeof err.message === 'string') {
    return err.message;
  }
  return 'Request failed';
}

export const RBACTemplate: Component = () => {
  const { t } = useI18n();
  const auth = useAuth();

  const user = () => auth.user();
  const permissions = createMemo(() => user()?.permissions || []);
  const isSuperadminWildcard = createMemo(() => permissions().includes('*'));

  // Interactive feedback state for demo buttons
  const [lastAction, setLastAction] = createSignal<string | null>(null);
  const [imperativeResult, setImperativeResult] = createSignal<string | null>(null);

  // API Demo state
  const [apiEndpoint, setApiEndpoint] = createSignal<string>('');
  const [apiLoading, setApiLoading] = createSignal<boolean>(false);
  const [apiResponse, setApiResponse] = createSignal<string | null>(null);
  const [apiError, setApiError] = createSignal<string | null>(null);
  const [apiLatency, setApiLatency] = createSignal<number | null>(null);

  const triggerAction = (perm: string) => {
    setLastAction(`Action triggered for permission: "${perm}" at ${new Date().toLocaleTimeString()}`);
  };

  const handleImperativeCheck = (perm: string) => {
    if (auth.hasPermission(perm)) {
      setImperativeResult(`auth.hasPermission('${perm}') returned TRUE. Capability granted.`);
    } else {
      setImperativeResult(`auth.hasPermission('${perm}') returned FALSE. Access denied.`);
    }
  };

  const callApiEndpoint = async (name: string, fn: () => Promise<unknown>) => {
    setApiEndpoint(name);
    setApiLoading(true);
    setApiResponse(null);
    setApiError(null);
    const start = performance.now();
    try {
      const data = await fn();
      const elapsed = Math.round(performance.now() - start);
      setApiLatency(elapsed);
      setApiResponse(JSON.stringify(data, null, 2));
    } catch (err: unknown) {
      const elapsed = Math.round(performance.now() - start);
      setApiLatency(elapsed);
      setApiError(getErrorMessage(err));
    } finally {
      setApiLoading(false);
    }
  };

  const getVerbIcon = (verb: Verb) => {
    switch (verb) {
      case 'read':
        return <IconEye class="size-3.5" />;
      case 'create':
        return <IconPlus class="size-3.5" />;
      case 'update':
        return <IconPencil class="size-3.5" />;
      case 'delete':
        return <IconTrash class="size-3.5" />;
    }
  };

  return (
    <div class="space-y-8 max-w-6xl mx-auto pb-12">
      {/* Page Header */}
      <div class="border-b border-border pb-5">
        <div class="flex items-center gap-3 mb-2">
          <div class="p-2 rounded-lg bg-primary/10 text-primary">
            <IconShieldCheck class="size-6" />
          </div>
          <div>
            <h1 class="text-2xl font-bold tracking-tight text-foreground">{t('rbacTemplate.title')}</h1>
            <p class="text-sm text-muted-foreground mt-0.5">
              {t('rbacTemplate.subtitle')}
            </p>
          </div>
        </div>
      </div>

      {/* SECTION 1: Current User Permissions */}
      <Card class="border-border shadow-sm">
        <CardHeader class="pb-3 border-b border-border/60">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <IconKey class="size-5 text-primary" />
              <CardTitle class="text-base font-semibold">{t('rbacTemplate.currentPermissions')}</CardTitle>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">Active Role:</span>
              <Badge variant="default" class="font-mono text-xs capitalize">
                {user()?.role || 'anonymous'}
              </Badge>
            </div>
          </div>
          <CardDescription class="text-xs">
            Inspected directly from active session (<code class="text-xs bg-muted px-1 py-0.5 rounded">useAuth().user()</code>).
          </CardDescription>
        </CardHeader>
        <CardContent class="p-4 space-y-4">
          <Show when={isSuperadminWildcard()}>
            <Alert class="bg-amber-500/10 border-amber-500/20 text-amber-700 dark:text-amber-400">
              <IconSparkles class="size-4 text-amber-600 dark:text-amber-400" />
              <AlertTitle class="text-xs font-semibold">Superadmin Wildcard Active (`*`)</AlertTitle>
              <AlertDescription class="text-xs">
                Your session includes the wildcard capability. All permission queries (<code class="font-mono text-xs">hasPermission(...)</code>) evaluate to <strong class="font-semibold">true</strong>.
              </AlertDescription>
            </Alert>
          </Show>

          <div>
            <div class="text-xs font-semibold text-muted-foreground mb-2 flex items-center justify-between">
              <span>Assigned Capabilities ({permissions().length}):</span>
              <span class="text-[11px] font-normal font-mono">
                {isSuperadminWildcard() ? 'Full Unrestricted Access' : `${permissions().length} permissions active`}
              </span>
            </div>
            <div class="flex flex-wrap gap-1.5 p-3 rounded-md bg-muted/40 border border-border/60">
              <Show
                when={permissions().length > 0}
                fallback={<span class="text-xs text-muted-foreground italic">No permissions assigned to current session.</span>}
              >
                <For each={permissions()}>
                  {(perm) => (
                    <Badge
                      variant={perm === '*' ? 'default' : 'secondary'}
                      class="font-mono text-xs px-2 py-0.5"
                    >
                      {perm}
                    </Badge>
                  )}
                </For>
              </Show>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* SECTION 2: Permission-Gated UI Showcase */}
      <Card class="border-border shadow-sm">
        <CardHeader class="pb-3 border-b border-border/60">
          <div class="flex items-center gap-2">
            <IconShield class="size-5 text-primary" />
            <CardTitle class="text-base font-semibold">{t('rbacTemplate.permissionDemo')}</CardTitle>
          </div>
          <CardDescription class="text-xs">
            Demonstrates both <strong>conditional hiding</strong> and <strong>disabled-fallback</strong> patterns for all 6 domain resources.
          </CardDescription>
        </CardHeader>
        <CardContent class="p-4 space-y-6">
          <Show when={lastAction()}>
            <div class="flex items-center gap-2 p-2.5 rounded-md bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs">
              <IconCheck class="size-4 shrink-0" />
              <span>{lastAction()}</span>
            </div>
          </Show>

          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <For each={Object.entries(PERMISSION_RESOURCES)}>
              {([resourceKey, resourceInfo]) => (
                <div class="rounded-lg border border-border bg-card p-4 space-y-3 flex flex-col justify-between">
                  <div>
                    <div class="flex items-center justify-between mb-1">
						<span class="font-semibold text-sm text-foreground">{resourceKey === 'accurate_account' ? t('roleManagement.accurateAccountResource') : resourceInfo.label}</span>
                      <code class="text-[10px] font-mono bg-muted px-1.5 py-0.5 rounded text-muted-foreground">
                        {resourceKey}
                      </code>
                    </div>
                    <p class="text-[11px] text-muted-foreground">
                      Resources and verbs defined in system dictionary.
                    </p>
                  </div>

                  <div class="space-y-2 pt-2 border-t border-border/60">
                    <span class="text-[10px] uppercase font-semibold tracking-wider text-muted-foreground block">
                      Gated Actions (Click to Test)
                    </span>
                    <div class="grid grid-cols-2 gap-1.5">
                      <For each={ALL_VERBS}>
                        {(verb) => {
                          const permString = `${resourceKey}:${verb}`;
                          const isSupported = (resourceInfo.verbs as readonly string[]).includes(verb);

                          return (
                            <Show
                              when={isSupported}
                              fallback={
                                <div class="h-8 border border-dashed border-border/40 rounded flex items-center justify-center text-[10px] text-muted-foreground/30">
                                  n/a
                                </div>
                              }
                            >
                              <PermissionGuard
                                permission={permString}
                                fallback={
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    disabled
                                    class="h-8 text-xs gap-1 opacity-50 cursor-not-allowed justify-start px-2"
                                    title={`Lacks permission "${permString}"`}
                                  >
                                    <IconLock class="size-3 text-muted-foreground shrink-0" />
                                    <span class="capitalize truncate">{verb}</span>
                                  </Button>
                                }
                              >
                                <Button
                                  variant="outline"
                                  size="sm"
                                  class="h-8 text-xs gap-1.5 justify-start px-2 hover:bg-primary hover:text-primary-foreground transition-colors"
                                  onClick={() => triggerAction(permString)}
                                >
                                  {getVerbIcon(verb)}
                                  <span class="capitalize truncate">{verb}</span>
                                </Button>
                              </PermissionGuard>
                            </Show>
                          );
                        }}
                      </For>
                    </div>
                  </div>
                </div>
              )}
            </For>
          </div>
        </CardContent>
      </Card>

      {/* SECTION 3: Guard Component & Helper Showcase */}
      <Card class="border-border shadow-sm">
        <CardHeader class="pb-3 border-b border-border/60">
          <div class="flex items-center gap-2">
            <IconCode class="size-5 text-primary" />
            <CardTitle class="text-base font-semibold">{t('rbacTemplate.guardDemo')}</CardTitle>
          </div>
          <CardDescription class="text-xs">
            Three core integration patterns for frontend developers.
          </CardDescription>
        </CardHeader>
        <CardContent class="p-4 space-y-5">
          {/* Pattern 1 */}
          <div class="rounded-lg border border-border p-4 bg-muted/20 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-foreground">
                Pattern 1: Declarative Component Guard (<code class="font-mono text-xs">&lt;PermissionGuard&gt;</code>)
              </span>
              <Badge variant="outline" class="text-[10px]">JSX Component</Badge>
            </div>
            <pre class="bg-muted p-2.5 rounded text-[11px] font-mono overflow-x-auto text-foreground">
              {`<PermissionGuard permission="design:create" fallback={<DisabledBtn />}>\n  <CreateButton onClick={handleCreate} />\n</PermissionGuard>`}
            </pre>
            <div class="pt-2 flex items-center gap-3">
              <span class="text-xs text-muted-foreground">Live result:</span>
              <PermissionGuard
                permission="design:create"
                fallback={<Badge variant="outline" class="text-xs text-destructive">Hidden / Fallback Rendered</Badge>}
              >
                <Badge variant="default" class="text-xs bg-emerald-600 hover:bg-emerald-600 text-white">
                  Visible (Permission Granted)
                </Badge>
              </PermissionGuard>
            </div>
          </div>

          {/* Pattern 2 */}
          <div class="rounded-lg border border-border p-4 bg-muted/20 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-foreground">
                Pattern 2: Imperative Hook Check (<code class="font-mono text-xs">auth.hasPermission(perm)</code>)
              </span>
              <Badge variant="outline" class="text-[10px]">Function Call</Badge>
            </div>
            <pre class="bg-muted p-2.5 rounded text-[11px] font-mono overflow-x-auto text-foreground">
              {`const auth = useAuth();\nif (!auth.hasPermission('role:update')) {\n  toast.error('Unauthorized');\n  return;\n}`}
            </pre>
            <div class="pt-2 flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                class="text-xs h-8"
                onClick={() => handleImperativeCheck('role:update')}
              >
                Test Check: "role:update"
              </Button>
              <Button
                size="sm"
                variant="outline"
                class="text-xs h-8"
                onClick={() => handleImperativeCheck('nonexistent:action')}
              >
                Test Check: "nonexistent:action"
              </Button>
            </div>
            <Show when={imperativeResult()}>
              <div class="mt-2 p-2 rounded bg-background border border-border text-xs font-mono text-foreground">
                {imperativeResult()}
              </div>
            </Show>
          </div>

          {/* Pattern 3 */}
          <div class="rounded-lg border border-border p-4 bg-muted/20 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-foreground">
                Pattern 3: Multi-Permission OR Check (<code class="font-mono text-xs">auth.hasAnyPermission(...perms)</code>)
              </span>
              <Badge variant="outline" class="text-[10px]">Compound Query</Badge>
            </div>
            <pre class="bg-muted p-2.5 rounded text-[11px] font-mono overflow-x-auto text-foreground">
              {`const canManage = auth.hasAnyPermission('user:create', 'user:update', 'user:delete');\n<Show when={canManage}><AdminToolbar /></Show>`}
            </pre>
            <div class="pt-2 flex items-center gap-3">
              <span class="text-xs text-muted-foreground">User Management Capability:</span>
              <Badge
                variant={auth.hasAnyPermission('user:create', 'user:update') ? 'default' : 'outline'}
                class="text-xs font-mono"
              >
                {auth.hasAnyPermission('user:create', 'user:update') ? 'User Management Capable' : 'No User Modification Permissions'}
              </Badge>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* SECTION 4: Role-Based vs Permission-Based Comparison */}
      <Card class="border-border shadow-sm">
        <CardHeader class="pb-3 border-b border-border/60">
          <CardTitle class="text-base font-semibold">Architectural Comparison: Role vs Permission</CardTitle>
          <CardDescription class="text-xs">
            Why the codebase migrated from coarse role checks to granular permissions.
          </CardDescription>
        </CardHeader>
        <CardContent class="p-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Old Pattern */}
            <div class="rounded-lg border border-destructive/20 bg-destructive/5 p-4 space-y-3">
              <div class="flex items-center gap-2 text-destructive font-semibold text-xs">
                <IconAlertCircle class="size-4" />
                <span>Old Pattern: Coarse Role Checking (Anti-Pattern)</span>
              </div>
              <pre class="bg-background/80 p-2.5 rounded text-[11px] font-mono border border-border text-foreground">
                {`// Breaks when custom roles are introduced\nconst isAdmin = auth.user()?.role === 'admin' ||\n                auth.user()?.role === 'superadmin';\n<Show when={isAdmin}>\n  <EditPresetButton />\n</Show>`}
              </pre>
              <ul class="text-xs text-muted-foreground space-y-1 list-disc list-inside">
                <li>Hardcodes role names across dozens of components</li>
                <li>Breaks when a custom role like "operator" needs print-only rights</li>
                <li>Requires frontend changes every time a role is added</li>
              </ul>
              <div class="pt-1 text-xs">
                <span class="text-muted-foreground">Current user matches 'admin': </span>
                <strong class="font-mono">{String(user()?.role === 'admin')}</strong>
              </div>
            </div>

            {/* New Pattern */}
            <div class="rounded-lg border border-emerald-500/20 bg-emerald-500/5 p-4 space-y-3">
              <div class="flex items-center gap-2 text-emerald-600 dark:text-emerald-400 font-semibold text-xs">
                <IconCheck class="size-4" />
                <span>New Pattern: Granular Permission Checking (Best Practice)</span>
              </div>
              <pre class="bg-background/80 p-2.5 rounded text-[11px] font-mono border border-border text-foreground">
                {`// Resilient to any system or custom role\n<PermissionGuard permission="preset:update">\n  <EditPresetButton />\n</PermissionGuard>`}
              </pre>
              <ul class="text-xs text-muted-foreground space-y-1 list-disc list-inside">
                <li>Decouples UI capabilities from role hierarchy</li>
                <li>Custom roles immediately get access without code changes</li>
                <li>Consistent with backend REST API permission enforcement</li>
              </ul>
              <div class="pt-1 text-xs">
                <span class="text-muted-foreground">Current user has 'preset:update': </span>
                <strong class="font-mono text-emerald-600 dark:text-emerald-400">
                  {String(auth.hasPermission('preset:update'))}
                </strong>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* SECTION 5: End-to-End API Integration */}
      <Card class="border-border shadow-sm">
        <CardHeader class="pb-3 border-b border-border/60">
          <div class="flex items-center gap-2">
            <IconApiApp class="size-5 text-primary" />
            <CardTitle class="text-base font-semibold">{t('rbacTemplate.apiDemo')}</CardTitle>
          </div>
          <CardDescription class="text-xs">
            Live execution against backend API endpoints. Each button verifies its required permission.
          </CardDescription>
        </CardHeader>
        <CardContent class="p-4 space-y-4">
          <div class="flex flex-wrap gap-2">
            <PermissionGuard permission="role:read">
              <Button
                variant="outline"
                size="sm"
                class="text-xs h-8"
                disabled={apiLoading()}
                onClick={() => void callApiEndpoint('GET /api/v1/admin/roles', getRoles)}
              >
                List Roles (role:read)
              </Button>

              <Button
                variant="outline"
                size="sm"
                class="text-xs h-8"
                disabled={apiLoading()}
                onClick={() => void callApiEndpoint('GET /api/v1/admin/permissions', getAllPermissions)}
              >
                List Permissions (role:read)
              </Button>

              <Button
                variant="outline"
                size="sm"
                class="text-xs h-8"
                disabled={apiLoading()}
                onClick={() => void callApiEndpoint('GET /api/v1/admin/roles/1/permissions', () => getRolePermissions(1))}
              >
                Get Role #1 Perms (role:read)
              </Button>
            </PermissionGuard>

            <PermissionGuard permission="user:read">
              <Button
                variant="outline"
                size="sm"
                class="text-xs h-8"
                disabled={apiLoading()}
                onClick={() => void callApiEndpoint('GET /api/v1/admin/users', getUsers)}
              >
                List Users (user:read)
              </Button>
            </PermissionGuard>

            <Button
              variant="outline"
              size="sm"
              class="text-xs h-8"
              disabled={apiLoading()}
              onClick={() => void callApiEndpoint('GET /auth/me', getMe)}
            >
              Session Me (/auth/me)
            </Button>
          </div>

          {/* Response Console */}
          <div class="rounded-lg border border-border bg-muted/30 overflow-hidden">
            <div class="bg-muted/70 px-3 py-2 border-b border-border/60 flex items-center justify-between text-xs">
              <span class="font-mono text-muted-foreground">
                {apiEndpoint() ? apiEndpoint() : 'No endpoint invoked yet'}
              </span>
              <div class="flex items-center gap-3">
                <Show when={apiLoading()}>
                  <span class="flex items-center gap-1 text-primary">
                    <IconLoader2 class="size-3.5 animate-spin" />
                    Executing...
                  </span>
                </Show>
                <Show when={apiLatency() !== null}>
                  <span class="text-[11px] font-mono text-muted-foreground">{apiLatency()}ms</span>
                </Show>
              </div>
            </div>

            <div class="p-3">
              <Show when={apiError()}>
                <div class="p-3 rounded bg-destructive/15 text-destructive border border-destructive/20 text-xs font-mono">
                  Error: {apiError()}
                </div>
              </Show>

              <Show when={apiResponse()}>
                <pre class="text-xs font-mono max-h-72 overflow-y-auto text-foreground">
                  {apiResponse()}
                </pre>
              </Show>

              <Show when={!apiResponse() && !apiError() && !apiLoading()}>
                <div class="text-xs text-muted-foreground italic py-6 text-center">
                  Click any endpoint button above to execute a live request and view the payload.
                </div>
              </Show>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};
