import type { ConsolePlugin } from "./modules/definePlugin";
import type { WidgetDefinition } from "./modules/dashboard/widgets/registry";

const moduleFiles = import.meta.glob("./modules/**/module.ts", {
  eager: true,
  import: "default",
}) as Record<string, ConsolePlugin>;

export const consolePlugins = Object.values(moduleFiles);

const contributedWidgets = new Map<string, { moduleId: string; widget: WidgetDefinition }>();
for (const plugin of consolePlugins) {
  for (const widget of plugin.widgets ?? []) contributedWidgets.set(widget.id, { moduleId: plugin.id, widget });
}

/** Sync compile-time widget contributions with the backend module manifest. */
export async function syncPluginWidgets(enabledModuleIds: ReadonlySet<string>): Promise<void> {
  const { widgetRegistry } = await import("./modules/dashboard/widgets/registry");
  for (const [id, contribution] of contributedWidgets) {
    const index = widgetRegistry.findIndex((widget) => widget.id === id);
    const enabled = enabledModuleIds.has(contribution.moduleId);
    if (enabled && index === -1) widgetRegistry.push(contribution.widget);
    if (!enabled && index !== -1) widgetRegistry.splice(index, 1);
  }
}

export function getConsolePlugin(id: string): ConsolePlugin | undefined {
  return consolePlugins.find((plugin) => plugin.id === id);
}
