<script lang="ts">
  import Cropper from 'cropperjs';
  import { onMount, onDestroy } from 'svelte';
  export let src = '';
  let img: HTMLImageElement;
  let cropper: Cropper;
  export function getDataURL() {
    const canvas = cropper.getCroppedCanvas({ width: 256, height: 256 });
    return canvas.toDataURL('image/png');
  }
  onMount(() => {
    cropper = new Cropper(img, {
      viewMode: 1,
      aspectRatio: 1,
      dragMode: 'move',
      guides: false,
      center: true,
      highlight: false,
      background: false,
      autoCropArea: 1,
    });
  });
  onDestroy(() => {
    cropper?.destroy();
  });
</script>

<style>
  @import 'cropperjs/dist/cropper.css';
  .cropper-view-box,
  .cropper-face {
    border-radius: 50%;
  }
</style>

<div class="w-full h-full flex items-center justify-center">
  <img bind:this={img} {src} alt="avatar" class="max-h-96" />
</div>
