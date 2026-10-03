export const managementPathFor = (context) => {
  const organization = context?.organizations?.find((item) => item.organizationId === context.activeOrganizationId);
  if (organization?.roles?.includes("app_ops_admin")) return "/ops/apps";
  if (organization?.roles?.includes("customer_success_admin")) return "/ops/organizations";
  return "";
};
