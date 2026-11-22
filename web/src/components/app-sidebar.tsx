import { ROUTES } from '~/routes';
import { For, Show, type Component, createMemo, onMount } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { A, useLocation, useNavigate } from '@solidjs/router';
import { useAuth } from '~/auth';
import { useI18n } from '~/i18n';
import {
  IconPackage,
  IconBarcode,
  IconUserCog,
  IconShieldCog,
  IconTestPipe,
  IconCloudComputing,
  IconPlugConnected,
  IconPlugConnectedX,
  IconLoader2,
  type IconProps,
} from '@tabler/icons-solidjs';
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from '~/components/ui/sidebar';
import {
  activeStatus,
  refreshStatus,
  workstationStatusState,
} from '~/stores/accurateAuth';

interface NavItem {
  href: string;
  label: string;
  icon: Component<IconProps>;
  exact?: boolean;
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

export const AppSidebar: Component = () => {
  const location = useLocation();
  const auth = useAuth();
  const { t } = useI18n();
  const sidebar = useSidebar();
  const { isMobile, setOpenMobile } = sidebar;

  // App mounts the authenticated sidebar only after the user (including
  // permissions) is loaded. Initialize once with cheap metadata, never validate.
  onMount(() => {
    if (auth.hasPermission('accurate:read')) void refreshStatus();
  });
  const navigate = useNavigate();
  const status = activeStatus;
  const showAccurateStatus = createMemo(
    () => auth.hasPermission('accurate:read') && workstationStatusState() !== 'denied'
  );
  const statusLabel = createMemo(() => {
    if (workstationStatusState() === 'loading') return t('accurate.sessionChecking');
    if (workstationStatusState() === 'failed') return t('accurate.statusUnavailable');
    const s = status();
    if (s.connected) {
      const account = s.accountName || s.database?.alias;
      return `${t('barcode.accurateConnected')}${account ? ` · ${account}` : ''}`;
    }
    if (s.enabled) return t('barcode.accurateDisconnected');
    return `Accurate · ${t('accurate.noActiveAccount')}`;
  });
  const statusColor = createMemo(() => {
    if (workstationStatusState() !== 'ready') return 'text-muted-foreground';
    const s = status();
    if (s.connected) return 'text-emerald-600 dark:text-emerald-400';
    if (s.enabled) return 'text-amber-600 dark:text-amber-400';
    return 'text-muted-foreground';
  });
  const statusIcon = createMemo(() => {
    if (workstationStatusState() === 'loading') return IconLoader2;
    if (workstationStatusState() === 'failed') return IconPlugConnectedX;
    return status().connected ? IconPlugConnected : IconPlugConnectedX;
  });
  const handleStatusClick = () => {
    handleNavClick();
    navigate(ROUTES.accurate);
  };

  const handleNavClick = () => {
    if (isMobile()) {
      setOpenMobile(false);
    }
  };

  const canManageUsers = createMemo(() => auth.hasPermission('user:read'));
  const canManageRoles = createMemo(() => auth.hasPermission('role:read'));
	const canViewAccurate = createMemo(() => auth.hasPermission('accurate_account:read'));

  const adminItems = createMemo<NavItem[]>(() => {
    const items: NavItem[] = [];
    if (canManageUsers()) items.push({ href: ROUTES.users, label: t('nav.userManagement'), icon: IconUserCog });
    if (canManageRoles()) items.push({ href: ROUTES.roles, label: t('nav.roleManagement'), icon: IconShieldCog });
    if (auth.hasPermission('role:read')) items.push({ href: ROUTES.rbacTemplate, label: t('nav.rbacTemplate'), icon: IconTestPipe });
    return items;
  });

  const currentNavGroups = createMemo<NavGroup[]>(() => [
    {
      label: t('navGroups.tools'),
      items: [
        { href: ROUTES.barcode, label: t('nav.barcodePrinter'), icon: IconBarcode },
      ],
    },
    ...(canViewAccurate()
      ? [
          {
            label: t('navGroups.integration'),
            items: [
              { href: ROUTES.accurate, label: t('nav.accurateAccount'), icon: IconCloudComputing },
            ],
          },
        ]
      : []),
    ...(adminItems().length > 0
      ? [
          {
            label: t('navGroups.administration'),
            items: adminItems(),
          },
        ]
      : []),
  ]);

  const isActive = (item: NavItem) =>
    item.exact
      ? location.pathname === item.href
      : location.pathname.startsWith(item.href);

  const NavLinks: Component<{ items: NavItem[] }> = (props) => (
    <SidebarMenu>
      <For each={props.items}>
        {(item) => {
          const Icon = item.icon;
          return (
            <SidebarMenuItem>
              <SidebarMenuButton
                as={A}
                href={item.href}
                isActive={isActive(item)}
                tooltip={item.label}
                onClick={handleNavClick}
              >
                <Icon class="size-4 shrink-0" />
                <span>{item.label}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          );
        }}
      </For>
    </SidebarMenu>
  );

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" class="cursor-default hover:bg-transparent">
              <div class="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <IconPackage class="size-4" />
              </div>
              <div class="grid flex-1 text-left text-sm leading-tight">
                <span class="truncate font-semibold">Warehouse Barcode</span>
                <span class="truncate text-xs text-muted-foreground">{t('common.adminConsole')}</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent class="group-data-[collapsible=icon]:overflow-y-auto group-data-[collapsible=icon]:overflow-x-hidden">
        <For each={currentNavGroups()}>
          {(group) => (
            <SidebarGroup>
              <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
              <SidebarGroupContent>
                <NavLinks items={group.items} />
              </SidebarGroupContent>
            </SidebarGroup>
          )}
        </For>
      </SidebarContent>

      <Show when={showAccurateStatus()}>
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                title={statusLabel()}
                aria-label={statusLabel()}
                onClick={handleStatusClick}
                class="cursor-pointer"
              >
                <span class="flex size-4 shrink-0 items-center justify-center">
                  <Dynamic component={statusIcon()} class={`size-4 shrink-0 ${statusColor()}`} />
                </span>
                <span class="truncate text-xs font-medium">{statusLabel()}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      </Show>

      <SidebarRail />
    </Sidebar>
  );
};
