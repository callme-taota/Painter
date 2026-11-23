import { computed, onBeforeUnmount, reactive, ref, type Ref } from 'vue';
import { api, APIError, type Row } from './api';
import { t } from './i18n';
import { toast } from './toast';
import { confirmAction } from './confirm';
import { randomID } from './random';
import { useMutationAttempt } from './useMutationAttempt';
// Homepage and Steam use one aggregate version. PSN is deliberately owned by
// a separate adapter; this module only coordinates the shared workbench lock.
export function useHomeSettings({ names, busy, locked, isGameWall, refreshItems, hasUnsavedChanges }: {
    names: Record<string, string>;
    busy: Ref<string>;
    locked: Readonly<Ref<boolean>>;
    isGameWall: () => boolean;
    refreshItems: () => Promise<unknown>;
    hasUnsavedChanges: () => boolean;
}) {
    const config = ref<Row>({}), version = ref(0), roles = ref(''), steam = reactive({ steam_id: '', mode: 'owned', auto_sync_enabled: false }), steamStatus = ref<Row>({}), apiKey = ref(''), clearKey = ref(false);
    const ready = ref(false), loadingHome = ref(true), homeInitial = ref(''), homeError = ref(''), homeConflict = ref(false), maxBytes = ref(10485760);
    const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value));
    const count = (value: unknown) => Array.from(String(value || '')).length;
    const configValue = (): Row => ({ ...clone(config.value), intro_roles: roles.value.split(/[、,，\n]/).map(s => s.trim()).filter(Boolean) });
    const steamValue = () => ({ steam_id: steam.steam_id, mode: steam.mode, auto_sync_enabled: steam.auto_sync_enabled });
    const homeSnapshot = () => JSON.stringify({ config: configValue(), steam: steamValue() });
    const homeAttemptSnapshot = () => JSON.stringify({ home: homeSnapshot(), api_key: apiKey.value, clear_api_key: clearKey.value });
    const saveAttempt = useMutationAttempt();
    const pendingHomeSave = computed(() => !!saveAttempt.pending.value);
    const homeDirty = computed(() => ready.value && (homeSnapshot() !== homeInitial.value || !!apiKey.value || clearKey.value || pendingHomeSave.value));
    const credentialsReady = computed(() => !!steam.steam_id && (!!apiKey.value || (!clearKey.value && !!steamStatus.value.api_key_configured)));
    let stopped = false, syncKey = randomID(), attemptSnapshot = '';
    function parseHome(data: Row) {
        const c = data?.config, s = data?.steam, keys = Object.keys(names);
        if (!Number.isSafeInteger(Number(data?.version)) || Number(data.version) < 1 || !c || !s || !Array.isArray(c.order) || c.order.length !== keys.length || new Set(c.order).size !== keys.length || c.order.some((k: string) => !keys.includes(k)) || keys.some(k => !c.sections?.[k] || typeof c.sections[k].enabled !== 'boolean' || !Number.isSafeInteger(c.sections[k].limit) || ['title', 'description', 'link_label', 'link_url'].some(f => typeof c.sections[k][f] !== 'string')) || !c.activity || !['posts_moments', 'posts', 'moments'].includes(c.activity.source) || !['rolling', 'year'].includes(c.activity.period) || typeof c.activity.timezone !== 'string' || !Number.isInteger(c.activity.year) || !Array.isArray(c.intro_roles) || c.intro_roles.some((r: unknown) => typeof r !== 'string') || (c.intro_card_name !== undefined && typeof c.intro_card_name !== 'string') || typeof c.use_site_intro !== 'boolean' || typeof c.motion_enabled !== 'boolean' || typeof c.warmth_enabled !== 'boolean' || ['intro_name', 'writing_title', 'writing_description', 'scroll_label'].some(k => typeof c[k] !== 'string') || typeof s.steam_id !== 'string' || typeof s.auto_sync_enabled !== 'boolean' || typeof s.api_key_configured !== 'boolean' || !['owned', 'recent'].includes(s.mode))
            throw new Error(t("首页设置未完整载入，当前修改已保留。"));
        return data;
    }
    function applyHome(data: Row, preserve = false) {
        parseHome(data);
        version.value = Number(data.version);
        steamStatus.value = clone(data.steam);
        homeInitial.value = JSON.stringify({ config: clone(data.config), steam: { steam_id: data.steam.steam_id, mode: data.steam.mode, auto_sync_enabled: !!data.steam.auto_sync_enabled } });
        if (!preserve) {
            config.value = clone(data.config);
            roles.value = data.config.intro_roles.join('、');
            Object.assign(steam, { steam_id: data.steam.steam_id, mode: data.steam.mode, auto_sync_enabled: !!data.steam.auto_sync_enabled });
            apiKey.value = '';
            clearKey.value = false;
        }
        const bytes = Number(data.max_image_bytes);
        if (Number.isSafeInteger(bytes) && bytes > 0)
            maxBytes.value = bytes;
        ready.value = true;
        homeConflict.value = false;
    }
    async function loadHome(replace = false) {
        if (locked.value)
            return;
        if (replace && homeDirty.value && !await confirmAction(t("载入服务器首页设置？"), t("当前页面内容和 Steam 设置修改将被替换，展示墙条目输入会保留。"), t("载入最新设置")))
            return;
        loadingHome.value = true;
        busy.value = 'read-home';
        homeError.value = '';
        try {
            const data = await api('/admin/home');
            if (!stopped) {
                applyHome(data);
                saveAttempt.clear();
            }
        }
        catch (e) {
            homeError.value = (e as Error).message;
        }
        finally {
            loadingHome.value = false;
            busy.value = '';
        }
    }
    function homeProblem() {
        if (!ready.value)
            return t("请先载入首页设置。");
        const c = configValue();
        if (!Number.isInteger(c.activity?.year) || (c.activity.year !== 0 && (c.activity.year < 2000 || c.activity.year > 2100)))
            return t("统计年份请选择0（当前年份）或2000至2100之间的年份。");
        for (const key of c.order) {
            const s = c.sections[key], label = names[key];
            if (count(s.title) > 80 || count(s.description) > 600 || count(s.link_label) > 60 || count(s.link_url) > 2048)
                return t("{value1}的文案或链接超出上限。", { value1: label });
            const max = key === 'articles' ? 12 : ['steam', 'music', 'photos', 'projects'].includes(key) ? 24 : 0;
            if (!Number.isInteger(Number(s.limit)) || Number(s.limit) < 0 || Number(s.limit) > max)
                return t("{value1}的显示数量不正确。", { value1: label });
        }
        if (count(c.intro_name) > 80 || count(c.intro_card_name) > 80 || c.intro_roles.length > 12 || c.intro_roles.some((r: string) => count(r) > 60) || count(c.writing_title) > 80 || count(c.writing_description) > 600 || count(c.scroll_label) > 60)
            return t("介绍或滚动文案超出上限，请检查后保存。");
        if (steam.steam_id && !/^\d{17}$/.test(steam.steam_id))
            return t("Steam ID64 须为17位数字。");
        if (steam.auto_sync_enabled && !credentialsReady.value)
            return t("自动同步需要先填写 Steam ID64 和 API Key。");
        return '';
    }
    async function saveHome() {
        if (locked.value || homeConflict.value || !homeDirty.value)
            return;
        const problem = pendingHomeSave.value ? '' : homeProblem();
        if (problem) {
            homeError.value = problem;
            return;
        }
        busy.value = 'save-home';
        homeError.value = '';
        try {
            const attempt = saveAttempt.begin(() => {
                attemptSnapshot = homeAttemptSnapshot();
                return { path: '/admin/home', method: 'PUT', version: version.value, payload: { config: configValue(), steam: { ...steamValue(), ...(apiKey.value ? { api_key: apiKey.value } : {}), ...(clearKey.value ? { clear_api_key: true } : {}) } } };
            });
            const result = await api(attempt.path, { method: attempt.method, version: attempt.version, key: attempt.key, body: attempt.body });
            if (stopped)
                return;
            const preserve = homeAttemptSnapshot() !== attemptSnapshot, payload = JSON.parse(attempt.body);
            applyHome(result, preserve);
            saveAttempt.clear();
            if (preserve) {
                if (apiKey.value === payload.steam.api_key)
                    apiKey.value = '';
                if (payload.steam.clear_api_key)
                    clearKey.value = false;
            }
            if (preserve)
                toast.warning(t("已确认上次保存。当前新修改仍未保存。"));
            else
                toast.success(t("首页设置已保存。"));
        }
        catch (e) {
            saveAttempt.failed(e);
            if (e instanceof APIError && [409, 412].includes(e.status)) {
                homeConflict.value = true;
                homeError.value = t("首页设置已在其他位置更新，当前修改已保留。请确认后载入最新设置。");
            }
            else if (e instanceof APIError && [400, 422].includes(e.status)) {
                saveAttempt.clear();
                homeError.value = e.message;
            }
            else
                toast.error((e as Error).message);
        }
        finally {
            busy.value = '';
        }
    }
    async function syncSteam() { if (locked.value || hasUnsavedChanges() || !credentialsReady.value || !ready.value)
        return; busy.value = 'sync'; homeError.value = ''; try {
        const data = await api('/admin/home/steam-sync', { method: 'POST', version: version.value, key: syncKey });
        if (!stopped) {
            applyHome(data);
            syncKey = randomID();
            toast.success(t("Steam 同步完成，本次处理 {value1} 项。", { value1: Number(data.synced_count) || 0 }));
            if (isGameWall())
                await refreshItems();
        }
    }
    catch (e) {
        if (e instanceof APIError && [409, 412].includes(e.status)) {
            homeConflict.value = true;
            homeError.value = e.message;
        }
        else
            toast.error((e as Error).message);
    }
    finally {
        busy.value = '';
    } }
    onBeforeUnmount(() => { stopped = true; saveAttempt.clear(); attemptSnapshot = ''; apiKey.value = ''; });
    return { config, roles, steam, steamStatus, apiKey, clearKey, ready, loadingHome, homeError, homeConflict, maxBytes, homeDirty, pendingHomeSave, credentialsReady, loadHome, saveHome, syncSteam };
}
