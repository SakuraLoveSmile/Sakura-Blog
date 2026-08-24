export type Breakpoint = "lg" | "md" | "sm" | "xs" | "xxs";

export interface DashboardWidget {
  i: string;
  id: string;
  x: number;
  y: number;
  w: number;
  h: number;
  minW?: number;
  minH?: number;
  maxW?: number;
  maxH?: number;
  config?: Record<string, unknown>;
  permissions?: string[];
}

export type DashboardResponsiveLayout = Partial<Record<Breakpoint, DashboardWidget[]>>;
