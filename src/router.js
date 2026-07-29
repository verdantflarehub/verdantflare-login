import { readonly, ref } from "vue";

const normalizePath = (value) => {
  const clean = (value || "/sign-in").split("?")[0].replace(/\/+$/, "");
  return clean || "/sign-in";
};

const path = ref(normalizePath(window.location.pathname));

const sync = () => {
  path.value = normalizePath(window.location.pathname);
  window.scrollTo({ top: 0, left: 0, behavior: "instant" });
};

window.addEventListener("popstate", sync);

export const currentPath = readonly(path);

export const navigate = (target) => {
  const url = new URL(target, window.location.origin);
  if (url.origin !== window.location.origin) return;
  window.history.pushState({}, "", `${url.pathname}${url.search}`);
  sync();
};
