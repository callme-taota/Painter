import { defineStore } from "pinia";
import { ref } from "vue";

export const useSearchStore = defineStore("searchStore", () => {
    const shows = ref(false)

    const onShow = () => {
        shows.value = true
    }

    const offShow = () => {
        shows.value = false
    }

    const doSearch = (value: string) => {

    }

    return { shows, offShow, doSearch, onShow }
})