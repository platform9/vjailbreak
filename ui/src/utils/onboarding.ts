export function shouldShowOnboarding(migrations: any, nodes: any, upgradeAvailable: boolean | undefined) {
  return (
    Array.isArray(migrations) && migrations.length === 0 &&
    (!nodes || nodes.length === 0) &&
    !upgradeAvailable
  );
} 