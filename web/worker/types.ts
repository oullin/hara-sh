export type Nullable<T> = T | null;

// One search hit, as POST /api/search/query returns it.
export type Result = { score: number; title: string; url: string };
