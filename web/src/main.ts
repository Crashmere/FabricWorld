import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import Library from "./pages/Library.vue";
import Editor from "./pages/Editor.vue";
import Detail from "./pages/Detail.vue";
import Remnant from "./pages/Remnant.vue";
import "./style.css";
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: "/", component: Library },
    { path: "/trash", component: Library },
    { path: "/new", component: Editor },
    { path: "/fabrics/:id", component: Detail },
    {
      path: "/fabrics/:id/edit",
      component: Editor,
      beforeEnter: (to) =>
        to.query.mode === "remnant"
          ? { path: `/fabrics/${to.params.id}/remnant` }
          : true,
    },
    { path: "/fabrics/:id/remnant", component: Remnant },
    { path: "/:pathMatch(.*)*", redirect: "/" },
  ],
  scrollBehavior(_to, _from, saved) {
    return saved || { top: 0 };
  },
});
createApp(App).use(router).mount("#app");
