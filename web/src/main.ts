import { createApp, createSSRApp } from 'vue';
import './style.css';
import App from './App.vue';

// Hydrate the published HTML; the development server starts with an empty root.
const app = document.querySelector('#app')?.hasChildNodes() ? createSSRApp(App) : createApp(App);

app.mount('#app');
