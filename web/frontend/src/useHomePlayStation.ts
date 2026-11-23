import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue';
import { api, APIError, body, type Row } from './api';
import { t } from './i18n';
import { toast } from './toast';
import { confirmAction } from './confirm';
import { randomID } from './random';
// PSN has its own version and lifecycle; never merge its credentials or writes
// into Steam settings. The parent supplies only view visibility and list refresh.
export function useHomePlayStation({ view, isGameWall, refreshItems }: {
    view: Ref<string>;
    isGameWall: () => boolean;
    refreshItems: () => unknown;
}) {
    let stopped = false;
    // PlayStation account state has its own version and never enters Steam's payload.
    const psn = ref<Row | null>(null), psnVersion = ref(0), psnReady = ref(false), psnLoading = ref(false), psnBusy = ref(''), psnError = ref(''), psnConflict = ref(false);
    const psnSecretAllowed = location.protocol === 'https:' || ['localhost', '127.0.0.1', '[::1]', '::1'].includes(location.hostname);
    const psnNpsso = ref(''), psnAuto = ref(false), psnOnlineID = ref(''), psnInitial = ref(''), psnPendingAction = ref('');
    const psnSnapshot = () => JSON.stringify({ auto_sync: psnAuto.value, online_id: psnOnlineID.value.trim() });
    const psnDirty = computed(() => psnReady.value && (!!psnNpsso.value || psnSnapshot() !== psnInitial.value || psnPendingAction.value === 'save'));
    const psnRunning = computed(() => psn.value?.status === 'queued' || psn.value?.status === 'syncing');
    const psnStateLabels: Record<string, string> = { idle: t("就绪"), queued: t("等待同步"), syncing: t("正在同步"), reauth_required: t("需要重新连接"), error: t("同步失败") };
    let psnTimer: ReturnType<typeof setTimeout> | undefined, psnReadController: AbortController | undefined, psnReadSequence = 0;
    const psnPollFailures=ref(0);
    let pendingPsnSave: {
        key: string;
        version: number;
        payload: Row;
    } | null = null, pendingPsnSync: {
        key: string;
        version: number;
    } | null = null, pendingPsnDisconnect: {
        key: string;
        version: number;
    } | null = null;
    function parsePsn(data: Row): Row {
        if (!data || !Number.isSafeInteger(data.version) || data.version < 1 || typeof data.connected !== 'boolean' || typeof data.online_id !== 'string' || typeof data.account_id !== 'string' || typeof data.auto_sync !== 'boolean' || !Object.keys(psnStateLabels).includes(data.status) || typeof data.last_error !== 'string' || !Number.isSafeInteger(data.game_count) || data.game_count < 0)
            throw new Error(t("PlayStation 状态未完整载入，请重试。"));
        return { version: data.version, connected: data.connected, online_id: data.online_id, account_id: data.account_id, auto_sync: data.auto_sync, status: data.status, last_sync_at: data.last_sync_at ?? null, last_error: data.last_error, next_sync_at: data.next_sync_at ?? null, game_count: data.game_count };
    }
    function psnSafeError(error: unknown) { let text = error instanceof Error ? error.message : t("请求失败，请稍后重试。"); for (const secret of [psnNpsso.value, pendingPsnSave?.payload.npsso])
        if (typeof secret === 'string' && secret)
            text = text.split(secret).join(t("[凭据已隐藏]")); return text; }
    function applyPsn(data: Row, replace = false) { const value = parsePsn(data), hadDirty = psnDirty.value; psn.value = value; if (!psnReady.value || replace || !hadDirty) {
        psnVersion.value = value.version;
        psnAuto.value = value.auto_sync;
        psnOnlineID.value = value.online_id;
        psnInitial.value = psnSnapshot();
        psnConflict.value = false;
        if (replace) {
            psnNpsso.value = '';
            pendingPsnSave = null;
            pendingPsnSync = null;
            pendingPsnDisconnect = null;
            psnPendingAction.value = '';
        }
    }
    else if (value.version !== psnVersion.value) {
        psnConflict.value = true;
        psnError.value = t("PlayStation 设置已在其他位置更新，当前输入已保留。请确认后载入最新状态。");
    } psnReady.value = true; }
    function stopPsnPoll() { clearTimeout(psnTimer); psnTimer = undefined; }
    function schedulePsnPoll() { stopPsnPoll(); if (!stopped && view.value === 'steam' && !document.hidden && psnRunning.value && !psnBusy.value && !psnLoading.value && psnPollFailures.value < 3)
        psnTimer = setTimeout(() => void loadPsn(false, true), 3000); }
    async function loadPsn(replace = false, automatic = false) {
        if (psnBusy.value || psnLoading.value || stopped)
            return;
        if (replace && psnDirty.value && !await confirmAction(t("载入最新 PlayStation 状态？"), t("当前未保存的 PlayStation 设置及本页输入的凭据会被放弃。Steam 设置和游戏条目不受影响。"), t("载入最新状态")))
            return;
        stopPsnPoll();
        const at = ++psnReadSequence;
        psnReadController?.abort();
        psnReadController = new AbortController();
        psnLoading.value = true;
        if (!automatic)
            psnError.value = '';
        try {
            const data = await api('/admin/home/psn', { signal: psnReadController.signal });
            if (stopped || at !== psnReadSequence)
                return;
            const previous = psn.value?.last_sync_at;
            applyPsn(data, replace);
            psnPollFailures.value = 0;
            if (previous !== undefined && data.last_sync_at !== previous && isGameWall())
                void refreshItems();
        }
        catch (error) {
            if (!stopped && at === psnReadSequence && (error as Error).name !== 'AbortError') {
                psnPollFailures.value++;
                psnError.value = psnSafeError(error) + (psnPollFailures.value >= 3 ? t(" 自动读取已暂停，可手动刷新状态。") : '');
            }
        }
        finally {
            if (at === psnReadSequence) {
                psnLoading.value = false;
                schedulePsnPoll();
            }
        }
    }
    function psnWriteError(error: unknown) { if (error instanceof APIError && (error.code === 'VERSION_CONFLICT' || error.status === 412)) {
        psnConflict.value = true;
        psnError.value = t("PlayStation 设置已更新，当前输入已保留。请载入最新状态后重试。");
        pendingPsnSave = null;
        pendingPsnSync = null;
        pendingPsnDisconnect = null;
        psnPendingAction.value = '';
    }
    else {
        if (error instanceof APIError && error.status < 500) {
            pendingPsnSave = null;
            pendingPsnSync = null;
            pendingPsnDisconnect = null;
            psnPendingAction.value = '';
        }
        if (error instanceof APIError && [400, 401, 403, 422].includes(error.status))
            psnError.value = psnSafeError(error);
        else
            toast.error(psnSafeError(error));
    } }
    async function savePsn() {
        if (!psnReady.value || psnBusy.value || psnLoading.value || psnConflict.value || !psnDirty.value || (psnPendingAction.value && psnPendingAction.value !== 'save'))
            return;
        if (!pendingPsnSave && !psn.value?.connected && !psnNpsso.value.trim()) {
            psnError.value = t("请先在自己的后台填入 NPSSO 凭据以连接账号。");
            return;
        }
        if (!psnSecretAllowed && (psnNpsso.value.trim() || pendingPsnSave?.payload.npsso)) {
            psnError.value = t("请通过 HTTPS 后台连接，或在本地服务器使用安全 CLI 填入凭据。");
            return;
        }
        const payload: Row = { auto_sync: psnAuto.value };
        if (psnNpsso.value.trim())
            payload.npsso = psnNpsso.value.trim();
        if (psnOnlineID.value.trim())
            payload.online_id = psnOnlineID.value.trim();
        pendingPsnSave ||= { key: randomID(), version: psnVersion.value, payload };
        const attempt = pendingPsnSave;
        psnPendingAction.value = 'save';
        stopPsnPoll();
        psnBusy.value = 'save';
        psnError.value = '';
        try {
            const data = await api('/admin/home/psn', { method: 'PUT', version: attempt.version, key: attempt.key, body: body(attempt.payload) });
            if (stopped)
                return;
            applyPsn(data, true);
            psnNpsso.value = '';
            pendingPsnSave = null;
            toast.success(t("PlayStation 设置已保存，凭据输入已清空。已保存的游戏不会因更新连接而删除。"));
        }
        catch (error) {
            if (!stopped)
                psnWriteError(error);
        }
        finally {
            psnBusy.value = '';
            schedulePsnPoll();
        }
    }
    async function syncPsn() {
        if (!psnReady.value || !psn.value?.connected || psnBusy.value || psnLoading.value || psnDirty.value || psnConflict.value || psnRunning.value || (psnPendingAction.value && psnPendingAction.value !== 'sync'))
            return;
        pendingPsnSync ||= { key: randomID(), version: psnVersion.value };
        const attempt = pendingPsnSync;
        psnPendingAction.value = 'sync';
        stopPsnPoll();
        psnBusy.value = 'sync';
        psnError.value = '';
        try {
            await api('/admin/home/psn/sync', { method: 'POST', version: attempt.version, key: attempt.key, body: body({}) });
            if (stopped)
                return;
            pendingPsnSync = null;
            psnPendingAction.value = '';
            toast.info(t("同步请求已提交，正在读取处理状态。"));
        }
        catch (error) {
            if (!stopped)
                psnWriteError(error);
        }
        finally {
            psnBusy.value = '';
            if (!stopped && !pendingPsnSync)
                await loadPsn(false, true);
            else
                schedulePsnPoll();
        }
    }
    async function disconnectPsn() {
        if (!psnReady.value || !psn.value?.connected || psnBusy.value || psnLoading.value || psnConflict.value || (psnPendingAction.value && psnPendingAction.value !== 'disconnect'))
            return;
        if (!pendingPsnDisconnect && !await confirmAction(t("断开 PlayStation 连接？"), t("将移除已保存的连接凭据并停止自动同步，本页未保存的 PlayStation 输入会放弃；现有游戏条目、封面与排序会保留。"), t("断开连接")))
            return;
        pendingPsnDisconnect ||= { key: randomID(), version: psnVersion.value };
        const attempt = pendingPsnDisconnect;
        psnPendingAction.value = 'disconnect';
        stopPsnPoll();
        psnBusy.value = 'disconnect';
        psnError.value = '';
        try {
            const data = await api('/admin/home/psn', { method: 'PUT', version: attempt.version, key: attempt.key, body: body({ auto_sync: false, disconnect: true }) });
            if (stopped)
                return;
            applyPsn(data, true);
            psnNpsso.value = '';
            pendingPsnDisconnect = null;
            toast.success(t("PlayStation 连接已断开，现有游戏条目已保留。"));
        }
        catch (error) {
            if (!stopped)
                psnWriteError(error);
        }
        finally {
            psnBusy.value = '';
            schedulePsnPoll();
        }
    }
    function psnVisibility() { if (document.hidden) {
        stopPsnPoll();
        psnReadController?.abort();
    }
    else if (view.value === 'steam')
        schedulePsnPoll(); }
    watch(view, value => { stopPsnPoll(); if (value === 'steam') {
        psnPollFailures.value = 0;
        void loadPsn(false, true);
    }
    else {
        psnReadSequence++;
        psnReadController?.abort();
        psnLoading.value = false;
    } });
    onMounted(() => document.addEventListener('visibilitychange', psnVisibility));
    onBeforeUnmount(() => { stopped = true; stopPsnPoll(); psnReadSequence++; psnReadController?.abort(); psnNpsso.value = ''; pendingPsnSave = null; pendingPsnSync = null; pendingPsnDisconnect = null; document.removeEventListener('visibilitychange', psnVisibility); });
    return { psn, psnReady, psnLoading, psnBusy, psnError, psnConflict, psnSecretAllowed, psnNpsso, psnAuto, psnOnlineID, psnPendingAction, psnDirty, psnRunning, psnStateLabels, loadPsn, savePsn, syncPsn, disconnectPsn, psnPollFailures, pendingPsnSync:computed(()=>psnPendingAction.value==='sync'), pendingPsnDisconnect:computed(()=>psnPendingAction.value==='disconnect'), pendingPsnSave: computed(() => psnPendingAction.value === 'save') };
}
