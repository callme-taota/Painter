import { defineStore } from "pinia";
import { ref } from "vue";

interface TitleGroup {
    primaryTitle: string,
    secondaryTitle: string,
}

export const titleStore = defineStore("titleStore", () => {
    const primaryTitle = ref("")
    const secondaryTitle = ref("")

    const setPrimaryTitle = (title: string) => {
        primaryTitle.value = title
    }

    const setSecondaryTitle = (title: string) => {
        secondaryTitle.value = title
    }

    const setTitle = (titleGroup: any | TitleGroup) => {
        primaryTitle.value = titleGroup.primaryTitle
        secondaryTitle.value = titleGroup.secondaryTitle
    }

    return { primaryTitle, secondaryTitle, setPrimaryTitle, setSecondaryTitle, setTitle }
})