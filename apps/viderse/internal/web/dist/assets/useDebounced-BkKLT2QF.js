import{r as o}from"./index-CSPxjcAP.js";function s(e,t=250){const[r,u]=o.useState(e);return o.useEffect(()=>{const c=setTimeout(()=>u(e),t);return()=>clearTimeout(c)},[e,t]),r}export{s as u};
