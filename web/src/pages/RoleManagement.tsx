import { type Component, createSignal, createResource, createMemo, Show, For, createEffect } from 'solid-js';
import { useI18n } from '~/i18n';
import { useAuth, PermissionGuard } from '~/auth';
import {
  getRoles,
  createRole,
  updateRole,
  deleteRole,
  setRolePermissions,
} from '~/api';
import type { Role } from '~/types';
import { PERMISSION_RESOURCES } from '~/domain';
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '~/components/ui/card';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from '~/components/ui/table';
import { Button } from '~/components/ui/button';
import { Badge } from '~/components/ui/badge';
import { TextField, TextFieldInput, TextFieldLabel } from '~/components/ui/text-field';
import { Checkbox, CheckboxControl } from '~/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '~/components/ui/dialog';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
} from '~/components/ui/alert-dialog';
import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import {
  IconSearch,
  IconPlus,
  IconPencil,
  IconTrash,
  IconCheck,
  IconAlertCircle,
  IconRefresh,
  IconShield,
  IconLock,
  IconShieldCheck,
} from '@tabler/icons-solidjs';

type Verb = 'read' | 'create' | 'update' | 'delete';
const ALL_VERBS: Verb[] = ['read', 'create', 'update', 'delete'];
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (err && typeof err === 'object' && 'message' in err && typeof err.message === 'string') {
    return err.message;
  }
  return 'An unexpected error occurred.';
}

function getErrorStatus(err: unknown): number | undefined {
  if (err && typeof err === 'object' && 'status' in err && typeof err.status === 'number') {
    return err.status;
  }
  return undefined;
}


export const RoleManagement: Component = () => {
  const { t } = useI18n();
  const auth = useAuth();
  const getVerbLabel = (verb: Verb): string => {
    switch (verb) {
      case 'read':
        return t('roleManagement.read');
      case 'create':
        return t('roleManagement.create');
      case 'update':
        return t('roleManagement.update');
      case 'delete':
        return t('roleManagement.delete');
    }
  };


  const [data, { refetch }] = createResource(getRoles);
  const [search, setSearch] = createSignal('');
  const [selectedRoleId, setSelectedRoleId] = createSignal<number | null>(null);

  // Notifications
  const [actionError, setActionError] = createSignal<string | null>(null);
  const [actionSuccess, setActionSuccess] = createSignal<string | null>(null);

  // Local permissions signal for selected role
  const [localPermissions, setLocalPermissions] = createSignal<string[]>([]);
  const [isSavingPermissions, setIsSavingPermissions] = createSignal(false);

  // Create role dialog
  const [isCreateOpen, setIsCreateOpen] = createSignal(false);
  const [createName, setCreateName] = createSignal('');
  const [createDescription, setCreateDescription] = createSignal('');
  const [createError, setCreateError] = createSignal<string | null>(null);
  const [isCreating, setIsCreating] = createSignal(false);

  // Edit role dialog
  const [isEditOpen, setIsEditOpen] = createSignal(false);
  const [roleToEdit, setRoleToEdit] = createSignal<Role | null>(null);
  const [editName, setEditName] = createSignal('');
  const [editDescription, setEditDescription] = createSignal('');
  const [editError, setEditError] = createSignal<string | null>(null);
  const [isEditing, setIsEditing] = createSignal(false);

  // Delete role dialog
  const [roleToDelete, setRoleToDelete] = createSignal<Role | null>(null);
  const [isDeleting, setIsDeleting] = createSignal(false);

  const clearNotifications = () => {
    setActionError(null);
    setActionSuccess(null);
  };

  const roles = createMemo(() => data()?.roles || []);

  const filteredRoles = createMemo(() => {
    const list = roles();
    const q = search().toLowerCase().trim();
    if (!q) return list;
    return list.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        (r.description && r.description.toLowerCase().includes(q))
    );
  });

  // Auto-select first role if none selected or if selected role was removed
  createEffect(() => {
    const list = roles();
    if (list.length > 0) {
      const current = selectedRoleId();
      if (current === null || !list.some((r) => r.id === current)) {
        setSelectedRoleId(list[0].id);
      }
    }
  });

  const selectedRole = createMemo(() => {
    const id = selectedRoleId();
    if (id === null) return null;
    return roles().find((r) => r.id === id) || null;
  });

  // Sync local permissions when selected role changes
  createEffect(() => {
    const role = selectedRole();
    if (role) {
      setLocalPermissions([...(role.permissions || [])]);
    } else {
      setLocalPermissions([]);
    }
  });

  const isDirty = createMemo(() => {
    const role = selectedRole();
    if (!role) return false;
    const current = [...localPermissions()].sort();
    const original = [...(role.permissions || [])].sort();
    if (current.length !== original.length) return true;
    return current.some((p, i) => p !== original[i]);
  });

  const isRoleProtected = createMemo(() => {
    const r = selectedRole();
    if (!r) return true;
    return r.name === 'superadmin' || r.name === 'guest';
  });

  const canEditPermissions = createMemo(() => {
    if (isRoleProtected()) return false;
    return auth.hasPermission('role:update');
  });

  const togglePermission = (perm: string) => {
    if (!canEditPermissions()) return;
    setLocalPermissions((prev) =>
      prev.includes(perm) ? prev.filter((p) => p !== perm) : [...prev, perm]
    );
  };

  const handleSelectAll = () => {
    if (!canEditPermissions()) return;
    const all: string[] = [];
    (Object.entries(PERMISSION_RESOURCES) as [string, { verbs: readonly string[] }][]).forEach(
      ([resKey, resVal]) => {
        resVal.verbs.forEach((v) => {
          all.push(`${resKey}:${v}`);
        });
      }
    );
    setLocalPermissions(all);
  };

  const handleDeselectAll = () => {
    if (!canEditPermissions()) return;
    setLocalPermissions([]);
  };

  const handleSavePermissions = async () => {
    const role = selectedRole();
    if (!role || !canEditPermissions()) return;
    clearNotifications();
    setIsSavingPermissions(true);
    try {
      await setRolePermissions(role.id, localPermissions());
      setActionSuccess(`Permissions for role "${role.name}" updated successfully.`);
      await refetch();
      await auth.refetch();
    } catch (err: unknown) {
      setActionError(getErrorMessage(err) || 'Failed to update role permissions.');
      setIsSavingPermissions(false);
    }
  };

  const handleResetPermissions = () => {
    const role = selectedRole();
    if (role) {
      setLocalPermissions([...(role.permissions || [])]);
    }
  };

  // Create role
  const openCreateDialog = () => {
    setCreateName('');
    setCreateDescription('');
    setCreateError(null);
    setIsCreateOpen(true);
  };

  const handleCreateSubmit = async (e: Event) => {
    e.preventDefault();
    const name = createName().trim();
    if (!name) {
      setCreateError('Role name is required.');
      return;
    }
    setCreateError(null);
    setIsCreating(true);
    try {
      const created = await createRole({
        name,
        description: createDescription().trim(),
        permissions: [],
      });
      setIsCreateOpen(false);
      setActionSuccess(`Role "${created.name}" created successfully.`);
      await refetch();
      setSelectedRoleId(created.id);
    } catch (err: unknown) {
      setCreateError(getErrorMessage(err) || 'Failed to create role.');
      setIsCreating(false);
    }
  };

  // Edit role
  const openEditDialog = (role: Role) => {
    setRoleToEdit(role);
    setEditName(role.name);
    setEditDescription(role.description || '');
    setEditError(null);
    setIsEditOpen(true);
  };

  const handleEditSubmit = async (e: Event) => {
    e.preventDefault();
    const role = roleToEdit();
    if (!role) return;
    const name = editName().trim();
    if (!role.is_system && !name) {
      setEditError('Role name is required.');
      return;
    }
    setEditError(null);
    setIsEditing(true);
    try {
      const updated = await updateRole(role.id, {
        name: role.is_system ? role.name : name,
        description: editDescription().trim(),
      });
      setIsEditOpen(false);
      setActionSuccess(`Role "${updated.name}" updated successfully.`);
      await refetch();
    } catch (err: unknown) {
      setEditError(getErrorMessage(err) || 'Failed to update role.');
      setIsEditing(false);
    }
  };

  // Delete role
  const confirmDelete = async () => {
    const target = roleToDelete();
    if (!target) return;
    clearNotifications();
    setIsDeleting(true);
    try {
      await deleteRole(target.id);
      setActionSuccess(`Role "${target.name}" deleted successfully.`);
      setRoleToDelete(null);
      if (selectedRoleId() === target.id) {
        setSelectedRoleId(null);
      }
      await refetch();
    } catch (err: unknown) {
      setActionError(
        getErrorStatus(err) === 409
          ? t('roleManagement.roleInUse')
          : getErrorMessage(err) || 'Failed to delete role.'
      );
      setRoleToDelete(null);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div class="space-y-6">
      {/* Top Header */}
      <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold tracking-tight text-foreground">{t('roleManagement.title')}</h1>
          <p class="text-sm text-muted-foreground mt-1">
            {t('roleManagement.subtitle')}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" class="gap-1.5" onClick={() => refetch()}>
            <IconRefresh class="h-4 w-4" />
            {t('common.refresh')}
          </Button>
          <PermissionGuard permission="role:create">
            <Button size="sm" class="gap-1.5" onClick={openCreateDialog}>
              <IconPlus class="h-4 w-4" />
              {t('roleManagement.createRole')}
            </Button>
          </PermissionGuard>
        </div>
      </div>

      {/* Action Alerts */}
      <Show when={actionSuccess()}>
        <div class="flex items-center gap-2 p-3 text-sm rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
          <IconCheck class="h-4 w-4 shrink-0" />
          <span>{actionSuccess()}</span>
        </div>
      </Show>

      <Show when={actionError()}>
        <div class="flex items-center gap-2 p-3 text-sm rounded-lg bg-destructive/15 text-destructive border border-destructive/20">
          <IconAlertCircle class="h-4 w-4 shrink-0" />
          <span>{actionError()}</span>
        </div>
      </Show>

      {/* Main Two-Panel Layout */}
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
        {/* Left Panel: Role List */}
        <div class="lg:col-span-5 space-y-4">
          <Card class="border-border shadow-sm">
            <CardHeader class="pb-3 border-b border-border/60">
              <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                <div>
                  <CardTitle class="text-base font-semibold">{t('roleManagement.rolesCard')}</CardTitle>
                  <CardDescription class="text-xs">
                    {filteredRoles().length} role{filteredRoles().length === 1 ? '' : 's'} total
                  </CardDescription>
                </div>
                <div class="w-full sm:w-48">
                  <TextField value={search()} onChange={setSearch}>
                    <div class="relative">
                      <IconSearch class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                      <TextFieldInput
                        placeholder={t('roleManagement.filterPlaceholder')}
                        class="h-9 pl-8 text-xs bg-background"
                      />
                    </div>
                  </TextField>
                </div>
              </div>
            </CardHeader>
            <CardContent class="p-0">
              <Table>
                <TableHeader>
                  <TableRow class="hover:bg-transparent">
                    <TableHead class="w-[120px]">{t('roleManagement.roleName')}</TableHead>
                    <TableHead class="w-[80px]">Type</TableHead>
                    <TableHead class="w-[70px] text-center">Perms</TableHead>
                    <TableHead class="text-right w-[90px]">{t('common.action')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <Show
                    when={!data.loading}
                    fallback={
                      <TableRow>
                        <TableCell colSpan={4} class="h-24 text-center text-sm text-muted-foreground">
                          {t('common.loading')}
                        </TableCell>
                      </TableRow>
                    }
                  >
                    <Show
                      when={filteredRoles().length > 0}
                      fallback={
                        <TableRow>
                          <TableCell colSpan={4} class="h-24 text-center text-sm text-muted-foreground">
                            {t('roleManagement.noRoles')}
                          </TableCell>
                        </TableRow>
                      }
                    >
                      <For each={filteredRoles()}>
                        {(role) => {
                          const isSelected = () => selectedRoleId() === role.id;
                          return (
                            <TableRow
                              class={`cursor-pointer transition-colors ${
                                isSelected()
                                  ? 'bg-primary/10 hover:bg-primary/15 font-medium'
                                  : 'hover:bg-muted/50'
                              }`}
                              onClick={() => setSelectedRoleId(role.id)}
                            >
                              <TableCell class="py-3">
                                <div class="flex items-center gap-2">
                                  <span class="text-sm text-foreground">{role.name}</span>
                                </div>
                                <Show when={role.description}>
                                  <p class="text-xs text-muted-foreground line-clamp-1">{role.description}</p>
                                </Show>
                              </TableCell>
                              <TableCell class="py-3">
                                <Badge variant={role.is_system ? 'secondary' : 'outline'} class="text-[10px] px-1.5 py-0">
                                  {role.is_system ? t('roleManagement.systemRole') : t('roleManagement.customRole')}
                                </Badge>
                              </TableCell>
                              <TableCell class="py-3 text-center">
                                <Badge variant="outline" class="text-xs font-mono">
                                  {role.permissions?.length || 0}
                                </Badge>
                              </TableCell>
                              <TableCell class="py-3 text-right" onClick={(e) => e.stopPropagation()}>
                                <div class="flex items-center justify-end gap-1">
                                  {/* Edit role button: allowed for custom roles, or description-edit for system roles */}
                                  <PermissionGuard permission="role:update">
                                    <Show when={role.name !== 'superadmin' && role.name !== 'guest'}>
                                      <Button
                                        variant="ghost"
                                        size="icon"
                                        class="h-8 w-8 text-muted-foreground hover:text-foreground"
                                        title={t('roleManagement.editRole')}
                                        onClick={() => openEditDialog(role)}
                                      >
                                        <IconPencil class="h-4 w-4" />
                                      </Button>
                                    </Show>
                                  </PermissionGuard>
                                  {/* Delete role button: only for custom roles */}
                                  <Show when={!role.is_system}>
                                    <PermissionGuard permission="role:delete">
                                      <Button
                                        variant="ghost"
                                        size="icon"
                                        class="h-8 w-8 text-destructive hover:bg-destructive/10"
                                        title={t('roleManagement.deleteRole')}
                                        onClick={() => setRoleToDelete(role)}
                                      >
                                        <IconTrash class="h-4 w-4" />
                                      </Button>
                                    </PermissionGuard>
                                  </Show>
                                </div>
                              </TableCell>
                            </TableRow>
                          );
                        }}
                      </For>
                    </Show>
                  </Show>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>

        {/* Right Panel: Permission Matrix */}
        <div class="lg:col-span-7">
          <Show
            when={selectedRole()}
            fallback={
              <Card class="border-border shadow-sm p-8 text-center text-muted-foreground">
                <IconShield class="h-10 w-10 mx-auto mb-2 opacity-40" />
                <p class="text-sm">Select a role to configure permissions.</p>
              </Card>
            }
          >
            {(role) => (
              <Card class="border-border shadow-sm">
                <CardHeader class="pb-3 border-b border-border/60">
                  <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                    <div>
                      <div class="flex items-center gap-2">
                        <CardTitle class="text-base font-semibold">{role().name}</CardTitle>
                        <Badge variant={role().is_system ? 'secondary' : 'outline'} class="text-[10px] px-1.5 py-0">
                          {role().is_system ? t('roleManagement.systemRole') : t('roleManagement.customRole')}
                        </Badge>
                        <Show when={isDirty()}>
                          <Badge variant="warning" class="text-[10px] px-1.5 py-0">
                            Unsaved Changes
                          </Badge>
                        </Show>
                      </div>
                      <CardDescription class="text-xs mt-0.5">
                        {role().description || 'No description provided.'}
                      </CardDescription>
                    </div>

                    <Show when={canEditPermissions()}>
                      <div class="flex items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          class="h-7 text-xs px-2"
                          onClick={handleSelectAll}
                        >
                          {t('roleManagement.selectAll')}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          class="h-7 text-xs px-2"
                          onClick={handleDeselectAll}
                        >
                          {t('roleManagement.deselectAll')}
                        </Button>
                      </div>
                    </Show>
                  </div>
                </CardHeader>

                <CardContent class="p-4 space-y-4">
                  {/* Protected Role Notice */}
                  <Show when={isRoleProtected()}>
                    <Alert class="bg-muted/50 border-muted-foreground/20">
                      <IconLock class="h-4 w-4" />
                      <AlertTitle class="text-xs font-semibold">Protected Role</AlertTitle>
                      <AlertDescription class="text-xs text-muted-foreground">
                        {t('roleManagement.cannotModifySystemPerms')}
                      </AlertDescription>
                    </Alert>
                  </Show>

                  {/* Read-Only Notice if user lacks role:update */}
                  <Show when={!isRoleProtected() && !auth.hasPermission('role:update')}>
                    <Alert class="bg-muted/50 border-muted-foreground/20">
                      <IconLock class="h-4 w-4" />
                      <AlertTitle class="text-xs font-semibold">Read Only</AlertTitle>
                      <AlertDescription class="text-xs text-muted-foreground">
                        You do not have permission to modify role permissions.
                      </AlertDescription>
                    </Alert>
                  </Show>

                  {/* Permission Matrix Grid/Table */}
                  <div class="rounded-md border border-border overflow-hidden">
                    <Table>
                      <TableHeader class="bg-muted/40">
                        <TableRow class="hover:bg-transparent">
                          <TableHead class="w-[180px] font-semibold">{t('roleManagement.resource')}</TableHead>
                          <For each={ALL_VERBS}>
                            {(verb) => (
                              <TableHead class="text-center font-semibold capitalize w-[80px]">
                                {getVerbLabel(verb)}
                              </TableHead>
                            )}
                          </For>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        <For each={Object.entries(PERMISSION_RESOURCES)}>
                          {([resKey, resVal]) => {
                            const allowedVerbs = resVal.verbs as readonly string[];
                            return (
                              <TableRow class="hover:bg-muted/30">
                                <TableCell class="font-medium text-xs text-foreground py-2.5">
                                  <div>{resVal.label}</div>
                                  <div class="text-[10px] text-muted-foreground font-mono">{resKey}</div>
                                </TableCell>
                                <For each={ALL_VERBS}>
                                  {(verb) => {
                                    const permString = `${resKey}:${verb}`;
                                    const isSupported = allowedVerbs.includes(verb);
                                    const isChecked = () => localPermissions().includes(permString);

                                    return (
                                      <TableCell class="text-center py-2.5">
                                        <Show
                                          when={isSupported}
                                          fallback={<span class="text-xs text-muted-foreground/40">—</span>}
                                        >
                                          <div class="flex justify-center">
                                            <Checkbox
                                              checked={isChecked()}
                                              onChange={() => togglePermission(permString)}
                                              disabled={!canEditPermissions()}
                                              class="cursor-pointer disabled:cursor-not-allowed"
                                            >
                                              <CheckboxControl class="data-[checked]:bg-primary data-[checked]:border-primary" />
                                            </Checkbox>
                                          </div>
                                        </Show>
                                      </TableCell>
                                    );
                                  }}
                                </For>
                              </TableRow>
                            );
                          }}
                        </For>
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>

                {/* Footer with Save/Reset Actions */}
                <Show when={canEditPermissions()}>
                  <CardFooter class="pt-2 pb-4 border-t border-border/60 flex items-center justify-between">
                    <span class="text-xs text-muted-foreground">
                      {localPermissions().length} permission{localPermissions().length === 1 ? '' : 's'} assigned
                    </span>
                    <div class="flex items-center gap-2">
                      <Show when={isDirty()}>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={handleResetPermissions}
                          disabled={isSavingPermissions()}
                        >
                          {t('common.reset')}
                        </Button>
                      </Show>
                      <Button
                        size="sm"
                        disabled={!isDirty() || isSavingPermissions()}
                        onClick={handleSavePermissions}
                        class="gap-1.5"
                      >
                        <Show
                          when={isSavingPermissions()}
                          fallback={<IconCheck class="h-4 w-4" />}
                        >
                          <IconRefresh class="h-4 w-4 animate-spin" />
                        </Show>
                        {isSavingPermissions() ? t('common.saving') : t('roleManagement.savePermissions')}
                      </Button>
                    </div>
                  </CardFooter>
                </Show>
              </Card>
            )}
          </Show>
        </div>
      </div>

      {/* Create Role Dialog */}
      <Dialog open={isCreateOpen()} onOpenChange={setIsCreateOpen}>
        <DialogContent class="sm:max-w-[425px]">
          <form onSubmit={handleCreateSubmit}>
            <DialogHeader>
              <DialogTitle>{t('roleManagement.createRole')}</DialogTitle>
              <DialogDescription>
                Create a new custom role. You can assign permissions once the role is created.
              </DialogDescription>
            </DialogHeader>

            <div class="space-y-4 py-4">
              <Show when={createError()}>
                <div class="p-2.5 text-xs rounded-md bg-destructive/15 text-destructive border border-destructive/20 flex items-center gap-2">
                  <IconAlertCircle class="h-4 w-4 shrink-0" />
                  <span>{createError()}</span>
                </div>
              </Show>

              <TextField value={createName()} onChange={setCreateName}>
                <TextFieldLabel class="text-xs font-semibold">{t('roleManagement.roleName')}</TextFieldLabel>
                <TextFieldInput
                  placeholder="e.g. warehouse-operator"
                  class="h-9 text-sm mt-1"
                  disabled={isCreating()}
                />
              </TextField>

              <TextField value={createDescription()} onChange={setCreateDescription}>
                <TextFieldLabel class="text-xs font-semibold">{t('roleManagement.roleDescription')}</TextFieldLabel>
                <TextFieldInput
                  placeholder="e.g. Floor operator with label print capabilities"
                  class="h-9 text-sm mt-1"
                  disabled={isCreating()}
                />
              </TextField>
            </div>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setIsCreateOpen(false)}
                disabled={isCreating()}
              >
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={isCreating() || !createName().trim()}>
                {isCreating() ? t('common.saving') : t('common.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit Role Dialog */}
      <Dialog open={isEditOpen()} onOpenChange={setIsEditOpen}>
        <DialogContent class="sm:max-w-[425px]">
          <form onSubmit={handleEditSubmit}>
            <DialogHeader>
              <DialogTitle>{t('roleManagement.editRole')}</DialogTitle>
              <DialogDescription>
                Update the role details. System role names cannot be renamed.
              </DialogDescription>
            </DialogHeader>

            <div class="space-y-4 py-4">
              <Show when={editError()}>
                <div class="p-2.5 text-xs rounded-md bg-destructive/15 text-destructive border border-destructive/20 flex items-center gap-2">
                  <IconAlertCircle class="h-4 w-4 shrink-0" />
                  <span>{editError()}</span>
                </div>
              </Show>

              <TextField value={editName()} onChange={setEditName}>
                <TextFieldLabel class="text-xs font-semibold">{t('roleManagement.roleName')}</TextFieldLabel>
                <TextFieldInput
                  class="h-9 text-sm mt-1"
                  disabled={Boolean(isEditing() || roleToEdit()?.is_system)}
                />
                <Show when={roleToEdit()?.is_system}>
                  <p class="text-[11px] text-muted-foreground mt-1">System role names are immutable.</p>
                </Show>
              </TextField>

              <TextField value={editDescription()} onChange={setEditDescription}>
                <TextFieldLabel class="text-xs font-semibold">{t('roleManagement.roleDescription')}</TextFieldLabel>
                <TextFieldInput
                  class="h-9 text-sm mt-1"
                  disabled={isEditing()}
                />
              </TextField>
            </div>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setIsEditOpen(false)}
                disabled={isEditing()}
              >
                {t('common.cancel')}
              </Button>
              <Button type="submit" disabled={isEditing()}>
                {isEditing() ? t('common.saving') : t('common.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Role Confirmation Dialog */}
      <AlertDialog open={Boolean(roleToDelete())} onOpenChange={(open) => !open && setRoleToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogTitle class="flex items-center gap-2">
            <IconAlertCircle class="h-5 w-5 text-destructive" />
            <span>{t('roleManagement.deleteRole')}</span>
          </AlertDialogTitle>
          <AlertDialogDescription class="py-2 text-sm">
            {t('roleManagement.confirmDeleteRole')} <strong>"{roleToDelete()?.name}"</strong>? This action cannot be undone.
          </AlertDialogDescription>
          <AlertDialogFooter>
            <Button
              variant="outline"
              onClick={() => setRoleToDelete(null)}
              disabled={isDeleting()}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={isDeleting()}
            >
              {isDeleting() ? t('common.deleting') : t('common.delete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
};
