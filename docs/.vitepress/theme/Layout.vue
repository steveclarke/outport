<script setup>
import DefaultTheme from 'vitepress/theme'
import HomeLayout from './HomeLayout.vue'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #home-hero-before>
      <HomeLayout />
    </template>
    <template #doc-after>
      <footer class="site-footer">
        MIT License &middot;
        <a href="https://github.com/steveclarke/outport" target="_blank" rel="noopener">GitHub</a>
        &middot; Built in outport <a href="https://en.wikipedia.org/wiki/Newfoundland_(island)" target="_blank" rel="noopener">Newfoundland</a>
        <a href="https://en.wikipedia.org/wiki/Flag_of_Newfoundland_and_Labrador" target="_blank" rel="noopener" title="Flag of Newfoundland and Labrador"><svg class="nfld-flag" viewBox="0 0 43200 21600" xmlns="http://www.w3.org/2000/svg"><path fill="#fff" d="m0 0h43200v21600H0z"/><path fill="#003865" d="m3159 900h13547v7822zm0 19800h13547v-7822zm-1559-948v-8152h14121zm0-17904v8152h14121z"/><path fill="#ba0c2f" d="m18706 8722V900h23094zm0 1154h17894l3000 924-3000 924H18706l1600-924zm0 3002v7822h23094zm17631 6922H19606v-5667zm0-18000H19606v5667z"/><path fill="#ffa400" d="m20199 10276h16341l1701 524-1701 524H20199l907-524z"/></svg></a>
        by <a href="https://github.com/steveclarke" target="_blank" rel="noopener">Steve Clarke</a>
      </footer>
    </template>
  </Layout>
</template>
