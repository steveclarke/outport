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
  </Layout>
</template>
