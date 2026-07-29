const authBase = (import.meta.env.VITE_AUTH_API_BASE || "/api/auth").replace(/\/$/, "");
const hubBase = (import.meta.env.VITE_HUB_URL || "https://hub.verdantflarehub.com").replace(/\/$/, "");
const useMock = import.meta.env.DEV && import.meta.env.VITE_USE_MOCK !== "false";

const wait = (ms = 650) => new Promise((resolve) => window.setTimeout(resolve, ms));

const request = async (path, options = {}) => {
  const response = await fetch(`${authBase}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...options.headers },
    ...options,
  });

  const body = response.status === 204 ? null : await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(body?.message || "请求未完成，请稍后重试");
  }
  return body;
};

export const safeReturnTo = (value) => {
  if (!value || typeof value !== "string") return "/";
  if (!value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return "/";
  try {
    const url = new URL(value, "https://hub.verdantflarehub.com");
    return `${url.pathname}${url.search}${url.hash}`;
  } catch {
    return "/";
  }
};

export const hubUrl = (returnTo) => `${hubBase}${safeReturnTo(returnTo)}`;

const mockEmail = (email) => email.trim().toLowerCase();

export const authApi = {
  async signIn(payload) {
    if (!useMock) return request("/sign-in", { method: "POST", body: JSON.stringify(payload) });
    await wait();
    if (payload.password.length < 8) throw new Error("邮箱或密码不正确");
    localStorage.setItem("vf_mock_profile", JSON.stringify({ email: mockEmail(payload.email) }));
    return { redirectTo: hubUrl(payload.returnTo) };
  },

  async signUp(payload) {
    if (!useMock) return request("/sign-up", { method: "POST", body: JSON.stringify(payload) });
    await wait(760);
    return { email: mockEmail(payload.email) };
  },

  async sendVerificationCode(email) {
    if (!useMock) return request("/verification-code", { method: "POST", body: JSON.stringify({ email }) });
    await wait(420);
    return { expiresIn: 60 };
  },

  async forgotPassword(email) {
    if (!useMock) return request("/forgot-password", { method: "POST", body: JSON.stringify({ email }) });
    await wait();
    return { email: mockEmail(email) };
  },

  async resetPassword(payload) {
    if (!useMock) return request("/reset-password", { method: "POST", body: JSON.stringify(payload) });
    await wait();
    return { success: true };
  },

  async resendVerification(email) {
    if (!useMock) return request("/verify-email/resend", { method: "POST", body: JSON.stringify({ email }) });
    await wait(420);
    return { success: true };
  },

  enterpriseSso(returnTo) {
    const target = new URL(`${authBase}/sso/authorize`, window.location.origin);
    target.searchParams.set("return_to", safeReturnTo(returnTo));
    if (useMock) return `${window.location.origin}/callback/enterprise?code=demo&state=verified&return_to=${encodeURIComponent(safeReturnTo(returnTo))}`;
    return target.toString();
  },

  async completeCallback(params) {
    if (!useMock) return request(`/callback?${params.toString()}`);
    await wait(720);
    if (!params.get("code") || params.get("state") !== "verified") throw new Error("登录回调校验失败");
    return { redirectTo: hubUrl(params.get("return_to")) };
  },

  async logout(returnTo) {
    if (!useMock) return request("/logout", { method: "POST", body: JSON.stringify({ returnTo: safeReturnTo(returnTo) }) });
    await wait(360);
    localStorage.removeItem("vf_mock_profile");
    return { success: true };
  },
};
