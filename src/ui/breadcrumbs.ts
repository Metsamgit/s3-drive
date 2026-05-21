import { el, clear } from "./dom.ts";

export function renderBreadcrumbs(
  container: HTMLElement,
  prefix: string,
  onNavigate: (p: string) => void,
): void {
  clear(container);

  const rootCrumb = el("span",
    { class: prefix === "" ? "crumb current" : "crumb", onclick: () => prefix !== "" && onNavigate("") },
    "🏠 racine",
  );
  container.appendChild(rootCrumb);

  if (prefix === "") return;

  const parts = prefix.replace(/\/$/, "").split("/");
  let acc = "";
  for (let i = 0; i < parts.length; i++) {
    acc += parts[i] + "/";
    container.appendChild(el("span", {}, " / "));
    const isLast = i === parts.length - 1;
    const target = acc;
    container.appendChild(
      el("span",
        {
          class: isLast ? "crumb current" : "crumb",
          onclick: () => !isLast && onNavigate(target),
        },
        parts[i],
      ),
    );
  }
}
