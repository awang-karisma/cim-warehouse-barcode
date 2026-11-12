import { ROUTES } from '~/routes';
import { type Component, For, createMemo } from 'solid-js';
import { A, useLocation } from '@solidjs/router';
import { useAuth } from '~/auth';
import { useI18n } from '~/i18n';
import {
  IconPackage,
  IconBarcode,
  IconUserCog,
  IconShieldCog,
  IconTestPipe,
  IconCloudComputing,
  type IconProps,
} from '@tabler/icons-solidjs';
import {
  Sidebar,
  SidebarContent,
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

      <SidebarRail />
    </Sidebar>
  );
};
