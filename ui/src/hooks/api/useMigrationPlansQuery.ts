import {
    useQuery,
    UseQueryOptions,
    UseQueryResult,
  } from "@tanstack/react-query";
  import { getMigrationPlans } from "src/api/migration-plans/migrationPlans";
  import { MigrationPlan } from "src/api/migration-plans/model";
  
  export const MIGRATION_PLANS_QUERY_KEY = ['migrationPlans'];
  
  type Options = Omit<UseQueryOptions<MigrationPlan[]>, "queryKey" | "queryFn">;
  
  export const useMigrationPlansQuery = (
    namespace = undefined,
    options: Options = {}
  ): UseQueryResult<MigrationPlan[]> => {
    return useQuery<MigrationPlan[]>({
      queryKey: [...MIGRATION_PLANS_QUERY_KEY, namespace],
      queryFn: async () => getMigrationPlans(namespace),
      staleTime: Infinity,
      refetchOnWindowFocus: true,
      ...options, // Override with custom options
    });
  };