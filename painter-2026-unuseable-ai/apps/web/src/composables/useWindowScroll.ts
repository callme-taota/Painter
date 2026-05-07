import { ref, onMounted, onUnmounted } from "vue";

/** 被动监听窗口滚动，用于视差与滚动驱动动画 */
export function useWindowScroll() {
  const scrollY = ref(0);
  const update = () => {
    scrollY.value = window.scrollY || document.documentElement.scrollTop;
  };
  onMounted(() => {
    update();
    window.addEventListener("scroll", update, { passive: true });
  });
  onUnmounted(() => window.removeEventListener("scroll", update));
  return { scrollY };
}
