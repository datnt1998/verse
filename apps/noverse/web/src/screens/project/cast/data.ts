// Nguồn dữ liệu của tab Nhân vật: bảng xưng hô (GAP-38) và Não truyện (GAP-39, chỉ dùng
// `chapters` + `entities[].appears` cho hàng “Xuất hiện” của D7).
import { getAddressTerms, getBrain, getCharacters } from '../../../api/client'
import { keys, useResource } from '../../../data'

export const useAddressTerms = (project: string, story: string) =>
  useResource(keys.story(project, story, 'address'), ({ signal }) => getAddressTerms(project, story, signal))

export const useBrain = (project: string, story: string) =>
  useResource(keys.story(project, story, 'brain'), ({ signal }) => getBrain(project, story, signal))

/** Nhân vật của truyện (Gói S: tab Truyện của Bàn viết cũng đọc ở đây, không dùng snapshot). */
export const useCharacters = (project: string, story: string) =>
  useResource(keys.story(project, story, 'characters'), ({ signal }) => getCharacters(project, story, signal))
