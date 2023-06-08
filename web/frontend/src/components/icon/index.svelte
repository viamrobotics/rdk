<script lang="ts">
  export let path: string;
  export let title = "";
  export let size: number | string = 18;
  export let rotate: number | string = 0;
  export let color: string = "#000";

  $: rotation = `rotate(${rotate} 12 12)`;

  const isNumeric = (value: string) => {
    return /^-{0,1}\d+$/u.test(value);
  };

  $: iconSize = isNumeric(String(size)) ? `${size}px` : size;
</script>

<svg width={iconSize} height={iconSize} viewBox="0 0 24 24" fill={color}>
  {#if title}
    <title>{title}</title>
  {/if}
  <g transform={rotation}>
    <path d={path} />
  </g>
</svg>
