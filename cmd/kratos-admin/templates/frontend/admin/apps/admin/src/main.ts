import { bootstrapAdminApp } from "@liujitcn/kratos-admin-core";
import adminModules from "./modules";

const globalState = JSON.parse(window.localStorage.getItem("admin-global") ?? "null") as {
  primary?: string;
  isDark?: boolean;
} | null;
if (globalState) {
  const dot = document.querySelectorAll<HTMLElement>(".dot i");
  const html = document.querySelector("html");
  if (globalState.primary) dot.forEach(item => (item.style.background = globalState.primary ?? ""));
  if (globalState.isDark && html) html.style.background = "#141414";
}

void bootstrapAdminApp({ modules: adminModules });
