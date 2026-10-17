const useAiDomain = typeof window !== "undefined" && window.location.hostname === "login.verdantflare.ai";

export const hubBaseUrl = useAiDomain
  ? "https://hub.verdantflare.ai"
  : (import.meta.env.VITE_HUB_URL || "https://hub.verdantflarehub.com");

export const wwwBaseUrl = useAiDomain
  ? "https://www.verdantflare.ai"
  : "https://www.verdantflarehub.com";
