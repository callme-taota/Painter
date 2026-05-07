import { ref } from "vue";

export const usePagination = () => {
  const cursor = ref("");
  const limit = ref(20);

  const updateCursor = (next: string) => {
    cursor.value = next;
  };

  return { cursor, limit, updateCursor };
};
