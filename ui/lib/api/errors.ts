export class ApiResponseError extends Error {
  code: string | undefined;
  params: Record<string, string> | undefined;

  constructor(body: {
    error?: string;
    code?: string;
    params?: Record<string, string>;
  }) {
    super(body.error ?? 'An error occurred');
    this.name = 'ApiResponseError';
    this.code = body.code;
    this.params = body.params;
  }
}

export async function throwApiError(res: Response): Promise<never> {
  const body = await res.json().catch(() => ({})) as {
    error?: string;
    code?: string;
    params?: Record<string, string>;
  };
  throw new ApiResponseError(body);
}
