"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Activity,
  ArrowLeftRight,
  Boxes,
  CreditCard,
  Database,
  DatabaseBackup,
  FileText,
  Fingerprint,
  KeyRound,
  LayoutDashboard,
  LogOut,
  Palette,
  ScrollText,
  Settings,
  ShieldCheck,
  Ticket,
  HardDrive,
  UserCog,
} from "lucide-react";
import { toast } from "sonner";

import { apiSend, type Features } from "@/lib/api";
import { useFeatures, useUser } from "@/app/(dashboard)/user-context";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
  useSidebar,
} from "@/components/ui/sidebar";

interface NavItem {
  title: string;
  href: string;
  icon: typeof LayoutDashboard;
  exact?: boolean;
  /** When set, the item is only shown if this feature flag is enabled. */
  feature?: keyof Features;
}

const NAV: NavItem[] = [
  { title: "Dashboard", href: "/", icon: LayoutDashboard, exact: true },
  { title: "Buckets", href: "/buckets", icon: Database },
  { title: "Storage backends", href: "/clusters", icon: Boxes },
  { title: "Access Keys", href: "/keys", icon: KeyRound },
  { title: "API Tokens", href: "/tokens", icon: Ticket },
  { title: "Members", href: "/members", icon: UserCog, feature: "multi_user" },
  { title: "Policies", href: "/policies", icon: ShieldCheck, feature: "rbac_enforced" },
  { title: "SCIM", href: "/scim-tokens", icon: Fingerprint, feature: "scim" },
  { title: "Branding", href: "/branding", icon: Palette, feature: "white_label" },
  { title: "Billing", href: "/billing", icon: CreditCard, feature: "billing" },
  { title: "Migrations", href: "/migrations", icon: ArrowLeftRight },
  { title: "Ops", href: "/ops", icon: Activity },
  { title: "Backups", href: "/backups", icon: DatabaseBackup },
  { title: "Audit", href: "/audit", icon: ScrollText },
  { title: "Docs", href: "/docs", icon: FileText },
  { title: "Settings", href: "/settings", icon: Settings },
];

function initials(name: string, email: string): string {
  const source = name?.trim() || email;
  const parts = source.split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return source.slice(0, 2).toUpperCase();
}

export function AppSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const user = useUser();
  const features = useFeatures();
  const { isMobile } = useSidebar();

  const nav = NAV.filter((item) => !item.feature || features[item.feature]);

  function isActive(href: string, exact?: boolean) {
    if (exact) return pathname === href;
    return pathname === href || pathname.startsWith(href + "/");
  }

  async function logout() {
    try {
      await apiSend<void>("POST", "/auth/logout");
    } catch {
      // Even if the call fails, fall through to the login page.
    }
    toast.success("Signed out");
    router.replace("/login");
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link href="/">
                <div className="bg-primary text-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                  <HardDrive className="size-4" />
                </div>
                <div className="flex flex-col gap-0.5 leading-none">
                  <span className="font-semibold">buktio</span>
                  <span className="text-muted-foreground text-xs">S3 control plane</span>
                </div>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Platform</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {nav.map((item) => (
                <SidebarMenuItem key={item.href}>
                  <SidebarMenuButton
                    asChild
                    isActive={isActive(item.href, item.exact)}
                    tooltip={item.title}
                  >
                    <Link href={item.href}>
                      <item.icon />
                      <span>{item.title}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  size="lg"
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                >
                  <Avatar className="size-8 rounded-lg">
                    <AvatarFallback className="rounded-lg">
                      {initials(user.full_name, user.email)}
                    </AvatarFallback>
                  </Avatar>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">
                      {user.full_name || user.email}
                    </span>
                    <span className="text-muted-foreground truncate text-xs">
                      {user.email}
                    </span>
                  </div>
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
                side={isMobile ? "bottom" : "right"}
                align="end"
                sideOffset={4}
              >
                <DropdownMenuLabel className="font-normal">
                  <div className="flex flex-col">
                    <span className="truncate text-sm font-medium">
                      {user.full_name || user.email}
                    </span>
                    <span className="text-muted-foreground truncate text-xs capitalize">
                      {user.role}
                    </span>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={logout}>
                  <LogOut />
                  Sign out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  );
}
