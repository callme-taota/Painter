export type ApiEnvelope<T> = {
  code: number;
  message: string;
  traceId: string;
  data: T;
};

export type RequestOptions = {
  baseUrl: string;
  headers?: Record<string, string>;
};

export const createClient = (opts: RequestOptions) => {
  const request = async <T>(path: string, init?: RequestInit): Promise<ApiEnvelope<T>> => {
    const res = await fetch(`${opts.baseUrl}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(opts.headers ?? {}),
        ...(init?.headers ?? {}),
      },
      credentials: "include",
    });
    if (!res.ok) {
      throw new Error(`request failed: ${res.status}`);
    }
    return (await res.json()) as ApiEnvelope<T>;
  };

  return {
    identityLogin: (body: { username: string; password: string }) =>
      request<{ userId: string; accessToken: string; expiresAt: string }>(`/identity/auth/login`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    checkLogin: (body: { accessToken: string }) =>
      request<{ loggedIn: boolean; userId?: string }>(`/identity/auth/check`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    identityRegister: (body: { userId: string; userName: string; email?: string; passwd: string }) =>
      request<{ created: boolean }>(`/identity/auth/register`, {
        method: "POST",
        body: JSON.stringify({
          UserID: body.userId,
          UserName: body.userName,
          Email: body.email ?? "",
          Passwd: body.passwd,
        }),
      }),
    listArticles: (cursor?: string, limit = 20) =>
      request<{ items: unknown[]; nextCursor?: string }>(
        `/content/articles?limit=${limit}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`
      ),
    getArticle: (articleId: string) =>
      request<{
        articleId: string;
        title: string;
        summary: string;
        content: string;
        authorId: string;
        categoryId: string;
        likeCount: number;
        updatedAt: string;
      }>(`/content/articles/get?articleId=${encodeURIComponent(articleId)}`),
    createArticle: (body: {
      title: string;
      summary?: string;
      content: string;
      categoryId: string;
      tagIds?: string[];
    }) =>
      request<{ articleId: string }>(`/content/articles`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    listConfigs: (namespace: string) =>
      request<{ namespace: string; items: Array<{ key: string; value: string; valueType: string; updatedAt: string }> }>(
        `/system/configs?namespace=${encodeURIComponent(namespace)}`
      ),
    getAnalyticsOverview: () =>
      request<{ qps: number; p95ms: number; cacheHitRatio: number; generatedAt: string; slowSQLThreshold: string }>(
        `/analytics/overview`
      ),
    getAnalyticsHistory: (limit = 30) =>
      request<{ items: Array<{ date: string; qps: number; p95ms: number; cacheHitRatio: number; visits: number }> }>(
        `/analytics/history/list?limit=${limit}`
      ),
    listComments: (articleId: string) =>
      request<{ items: Array<{ commentId: string; articleId: string; userId: string; content: string; createdAt: string }> }>(
        `/content/comments?articleId=${encodeURIComponent(articleId)}`
      ),
    createComment: (body: { articleId: string; content: string }) =>
      request<{ commentId: string }>(`/content/comments`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    likeArticle: (body: { articleId: string }) =>
      request<{ success: boolean }>(`/content/interactions/like`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    collectArticle: (body: { articleId: string }) =>
      request<{ success: boolean }>(`/content/interactions/collect`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    listFollowings: (userId: string) =>
      request<{ items: Array<{ userId: string; nickname: string }> }>(
        `/identity/follows?userId=${encodeURIComponent(userId)}`
      ),
    listFollowers: (userId: string) =>
      request<{ items: Array<{ userId: string; nickname: string }> }>(
        `/identity/followers?userId=${encodeURIComponent(userId)}`
      ),
    createFollow: (body: { targetUserId: string }) =>
      request<{ success: boolean }>(`/identity/follows`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    getSelf: () =>
      request<{ userId: string; userName: string; email: string; nickName: string; phone: string; group: number }>(
        `/identity/users/self`
      ),
    getUserInfo: (userId: string) =>
      request<{ userId: string; userName: string; email: string; nickName: string; phone: string; group: number }>(
        `/identity/users/info?UserID=${encodeURIComponent(userId)}`
      ),
    listTags: () =>
      request<{ items: Array<{ tagId: number; name: string; desc: string; count: number }> }>(`/content/tags/list`),
    createTag: (body: { name: string; desc?: string }) =>
      request<{ success: boolean }>(`/content/tags/create`, {
        method: "POST",
        body: JSON.stringify({ Name: body.name, Desc: body.desc ?? "" }),
      }),
    deleteTag: (body: { tagId: number }) =>
      request<{ success: boolean }>(`/content/tags/delete`, {
        method: "POST",
        body: JSON.stringify({ TagID: body.tagId }),
      }),
    listCategories: () =>
      request<{ items: Array<{ categoryId: number; name: string; desc: string }> }>(`/content/categories/list`),
    createCategory: (body: { name: string; desc?: string }) =>
      request<{ success: boolean }>(`/content/categories/create`, {
        method: "POST",
        body: JSON.stringify({ Name: body.name, Desc: body.desc ?? "" }),
      }),
    deleteCollection: (body: { articleId: string; userId: string }) =>
      request<{ success: boolean }>(`/content/collection/delete`, {
        method: "POST",
        body: JSON.stringify({ ArticleID: body.articleId, UserID: body.userId }),
      }),
    getUserList: () =>
      request<{ items: Array<{ userId: string; userName: string; email: string; nickName: string; group: number }> }>(
        `/identity/admin/userlist`
      ),
    setUserPermission: (body: { userId: string; group: number }) =>
      request<{ success: boolean }>(`/identity/admin/user/permission`, {
        method: "POST",
        body: JSON.stringify({ UserID: body.userId, Group: body.group }),
      }),
    getAdminSettings: () =>
      request<{ siteName: string; icpCode?: string; github?: string; canRegister: boolean }>(`/system/admin/settings`),
    updateAdminSettings: (body: { siteName?: string; icpCode?: string; github?: string; canRegister?: boolean }) =>
      request<{ siteName: string; icpCode?: string; github?: string; canRegister: boolean }>(`/system/admin/settings`, {
        method: "PATCH",
        body: JSON.stringify(body),
      }),
  };
};
