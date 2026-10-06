/* Shared fetch stub for auth tests: routes by path and records every request. */

export type RecordedRequest = {
  method: string;
  path: string;
  authorization: string | null;
  body: string;
};

type Handler = (request: RecordedRequest) => Response | Promise<Response>;

export function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

export function installFetchStub(handler: Handler): RecordedRequest[] {
  const calls: RecordedRequest[] = [];
  globalThis.fetch = jest.fn(async (input: RequestInfo | URL) => {
    const request = input as Request;
    const recorded: RecordedRequest = {
      method: request.method,
      path: new URL(request.url).pathname,
      authorization: request.headers.get('Authorization'),
      body: request.method === 'GET' || request.method === 'HEAD' ? '' : await request.text(),
    };
    calls.push(recorded);
    return handler(recorded);
  }) as typeof fetch;
  return calls;
}

export function tokens(suffix: string, accessToken = `access-${suffix}`) {
  return {
    accessToken,
    refreshToken: `refresh-${suffix}`,
    tokenType: 'Bearer',
    expiresIn: 900,
  };
}

/** Unsigned JWT-shaped token with the given `exp` (the app only reads `exp`). */
export function jwtWithExp(expSeconds: number, label: string): string {
  // Node provides btoa; RN's type definitions don't declare it.
  const { btoa } = globalThis as unknown as { btoa: (data: string) => string };
  const encode = (value: object) =>
    btoa(JSON.stringify(value)).replace(/\+/g, '-').replace(/\//g, '_').replace(/[=]+$/, '');
  return `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ exp: expSeconds, sub: label })}.sig`;
}

export const USER = {
  id: 'user-1',
  email: 'athlete@atlas.local',
  isPro: false,
  entitlements: [],
  coachTier: 'free',
  createdAt: '2026-01-01T00:00:00.000Z',
};
