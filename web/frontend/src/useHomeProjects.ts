import { computed, nextTick, onBeforeUnmount, reactive, ref, type Ref } from 'vue';
import { api, APIError, body, list, type Row } from './api';
import { t } from './i18n';
import { toast } from './toast';
import { confirmAction } from './confirm';
import { randomID } from './random';
// Keep project overrides, visibility and command keys within this editor.
// List refreshes never replace the form without its existing discard check.
export function useHomeProjects({ busy, locked, uid }: {
    busy: Ref<string>;
    locked: Readonly<Ref<boolean>>;
    uid: string;
}) {
    let stopped = false;
    const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value));
    function rowMutationError(e: unknown) { const message = (e as Error).message; if (e instanceof APIError && [409, 412].includes(e.status))
        projectListError.value = t("{value1} 请刷新项目列表后重试。", { value1: message });
    else
        toast.error(message); }
    const projects = ref<Row[]>([]), projectsLoading = ref(false), projectError = ref(''), projectListError = ref(''), projectConflict = ref(false), project = reactive<Row>({}), projectInitial = ref('');
    const projectActions = new Map<string, string>();
    let pendingProject: {
        key: string;
        payload: Row;
        id: string;
        version: number;
    } | null = null;
    function projectValue(value: Row = project): Row { return { source: value.source, repo_url: value.source === 'github' ? value.repo_url : '', title: value.source === 'manual' || value.custom_title ? value.title : null, description: value.source === 'manual' || value.custom_description ? value.description : null, url: value.source === 'manual' || value.custom_url ? value.url : null, language: value.source === 'manual' || value.custom_language ? value.language : null, position: value.position, visible: value.visible }; }
    const projectDirty = computed(() => JSON.stringify(projectValue()) !== projectInitial.value);
    function fillProject(row?: Row) { Object.assign(project, { id: '', version: 0, source: 'github', repo_url: '', title: '', description: '', url: '', language: '', position: 0, visible: false, custom_title: false, custom_description: false, custom_url: false, custom_language: false }, row || {}); if (row) {
        for (const key of ['title', 'description', 'url', 'language'])
            project['custom_' + key] = row[key + '_override'] !== null;
    } projectInitial.value = JSON.stringify(projectValue()); projectConflict.value = false; pendingProject = null; }
    fillProject();
    async function loadProjects() { if (projectsLoading.value)
        return; projectsLoading.value = true; projectListError.value = ''; try {
        const data = await api('/admin/home/projects');
        if (!stopped) {
            const rows = list(data);
            if (rows.some(r => !String(r.id || '') || !Number.isSafeInteger(r.version) || r.version < 1 || !['github', 'manual'].includes(r.source) || typeof r.visible !== 'boolean'))
                throw new Error(t("项目列表未完整载入，请重试。"));
            projects.value = rows;
        }
    }
    catch (e) {
        if (!stopped)
            projectListError.value = (e as Error).message;
    }
    finally {
        projectsLoading.value = false;
    } }
    async function editProject(row?: Row) { if (locked.value)
        return; if (projectDirty.value && !await confirmAction(t("载入项目？"), t("当前未保存的项目修改将被替换。"), t("载入项目")))
        return; fillProject(row); projectError.value = ''; await nextTick(); document.getElementById(uid + '-project-source')?.focus(); }
    async function saveProject() { if (locked.value || !projectDirty.value || projectConflict.value)
        return; const payload = projectValue(); if (payload.source === 'github' && !payload.repo_url.trim()) {
        projectError.value = t("请填写公开 GitHub 仓库链接或 owner/repo。");
        return;
    } if (payload.source === 'manual' && (!payload.title?.trim() || !payload.url?.trim())) {
        projectError.value = t("请填写项目标题和链接。");
        return;
    } busy.value = 'save-project'; projectError.value = ''; const attempt = pendingProject || { key: randomID(), payload: clone(payload), id: project.id, version: project.version }; pendingProject = attempt; try {
        const row = await api('/admin/home/projects' + (attempt.id ? '/' + attempt.id : ''), { method: attempt.id ? 'PUT' : 'POST', key: attempt.key, version: attempt.version, body: body(attempt.payload) });
        if (!stopped) {
            fillProject(row);
            await loadProjects();
            toast.success(row.source === 'github' ? t("项目已保存。点击「刷新资料」可从 GitHub 读取公开信息。") : t("项目已保存。"));
        }
    }
    catch (e) {
        if (e instanceof APIError && [409, 412].includes(e.status)) {
            projectConflict.value = true;
            pendingProject = null;
            projectError.value = e.message;
        }
        else {
            if (e instanceof APIError && e.status < 500)
                pendingProject = null;
            if (e instanceof APIError && [400, 422].includes(e.status))
                projectError.value = e.message;
            else
                toast.error((e as Error).message);
        }
    }
    finally {
        busy.value = '';
    } }
    function projectActionKey(action: string, row: Row) { const id = action + ':' + row.id + ':' + row.version; if (!projectActions.has(id))
        projectActions.set(id, randomID()); return projectActions.get(id)!; }
    async function projectAction(row: Row, action: 'refresh' | 'delete') { if (locked.value)
        return; if (project.id === row.id && projectDirty.value) {
        projectError.value = t("请先保存或放弃当前项目修改。");
        return;
    } if (action === 'delete' && !await confirmAction(t("删除这个项目？"), t("{value1} 将从项目库移除。", { value1: row.title }), t("删除项目")))
        return; busy.value = action + '-project'; projectListError.value = ''; try {
        const data = await api('/admin/home/projects/' + row.id + (action === 'refresh' ? '/refresh' : ''), { method: action === 'refresh' ? 'POST' : 'DELETE', key: projectActionKey(action, row), version: row.version });
        if (!stopped) {
            if (action === 'delete' && project.id === row.id)
                fillProject();
            if (action === 'refresh' && project.id === row.id)
                fillProject(data);
            await loadProjects();
            if (action === 'delete')
                toast.success(t("项目已删除。"));
            else if (data.last_error)
                toast.warning(data.last_error);
            else
                toast.success(t("GitHub 资料已刷新，自定义文字已保留。"));
        }
    }
    catch (e) {
        rowMutationError(e);
    }
    finally {
        busy.value = '';
    } }
    async function projectVisibility(row: Row, event: Event) { const input = event.target as HTMLInputElement; if (locked.value || (project.id === row.id && projectDirty.value)) {
        input.checked = row.visible;
        return;
    } busy.value = 'project-visibility'; try {
        const value = { ...row };
        for (const key of ['title', 'description', 'url', 'language'])
            value['custom_' + key] = row[key + '_override'] !== null;
        const data = await api('/admin/home/projects/' + row.id, { method: 'PUT', version: row.version, key: projectActionKey('visible-' + input.checked, row), body: body({ ...projectValue(value), visible: input.checked }) });
        if (!stopped) {
            if (project.id === row.id)
                fillProject(data);
            await loadProjects();
            toast.success(data.visible ? t("项目已设为显示。") : t("项目已设为隐藏。"));
        }
    }
    catch (e) {
        input.checked = row.visible;
        rowMutationError(e);
    }
    finally {
        busy.value = '';
    } }
    async function reloadProject() { await loadProjects(); const row = projects.value.find(r => r.id === project.id); if (row)
        await editProject(row);
    else
        projectError.value = t("列表中已找不到这个项目，当前输入已保留。"); }
    onBeforeUnmount(() => { stopped = true; pendingProject = null; projectActions.clear(); });
    return { projects, projectsLoading, projectError, projectListError, projectConflict, project, projectDirty, loadProjects, editProject, saveProject, projectAction, projectVisibility, reloadProject };
}
